package mongorepo

import (
	"context"
	"errors"

	"tagowl/backend/internal/catalog"
)

func (r *Repository) List(ctx context.Context, filter catalog.StickerFilter) ([]catalog.Sticker, error) {
	normalized := catalog.NormalizeFilter(filter)
	return r.fetchOptimizedPublicStickers(ctx, buildPublicStickerFilter(normalized.Category, normalized.Tag), normalized.Sort, normalized.Limit)
}

func (r *Repository) GetByID(ctx context.Context, id string) (catalog.Sticker, bool, error) {
	sticker, ok, err := r.fetchStickerByID(ctx, id, false)
	if err != nil || !ok {
		return catalog.Sticker{}, ok, err
	}

	items, err := r.attachMetrics(ctx, []catalog.Sticker{sticker})
	if err != nil {
		return catalog.Sticker{}, false, err
	}
	return items[0], true, nil
}

func (r *Repository) Home(ctx context.Context, limit int) (catalog.HomeResponse, error) {
	normalizedLimit := limit
	if normalizedLimit <= 0 {
		normalizedLimit = catalog.DefaultHomeLimit
	}
	if normalizedLimit > catalog.MaxListLimit {
		normalizedLimit = catalog.MaxListLimit
	}

	categoryItems, err := r.ListCategories(ctx)
	if err != nil {
		return catalog.HomeResponse{}, err
	}
	categories := make([]string, 0, len(categoryItems))
	for _, category := range categoryItems {
		categories = append(categories, category.Name)
	}
	if len(categories) == 0 {
		categories, err = r.fetchDistinctPublicStickerCategories(ctx)
		if err != nil {
			return catalog.HomeResponse{}, err
		}
	}

	filter := buildPublicStickerFilter("", "")

	topTrending, err := r.fetchOptimizedPublicStickers(ctx, filter, "trending", publicHomeCandidateLimit(normalizedLimit))
	if err != nil {
		return catalog.HomeResponse{}, err
	}
	if len(topTrending) == 0 {
		return catalog.HomeResponse{}, errors.New("catalog is empty")
	}
	topTrending = limitWithCategoryDiversity(topTrending, normalizedLimit, trendingCategoryCap)

	newArrivals, err := r.fetchOptimizedPublicStickers(ctx, filter, "newest", normalizedLimit)
	if err != nil {
		return catalog.HomeResponse{}, err
	}

	topRated, err := r.fetchOptimizedPublicStickers(ctx, filter, "top_rated", normalizedLimit)
	if err != nil {
		return catalog.HomeResponse{}, err
	}

	return catalog.HomeResponse{
		Categories: categories,
		Sections: []catalog.HomeSection{
			{Key: "top-trending", Title: "Top Trending", Stickers: topTrending},
			{Key: "new-arrivals", Title: "New Arrivals", Stickers: newArrivals},
			{Key: "top-rated", Title: "Top Rated", Stickers: topRated},
		},
	}, nil
}

func publicHomeCandidateLimit(limit int) int {
	candidateLimit := limit * 5
	if candidateLimit < 40 {
		candidateLimit = 40
	}
	if candidateLimit > catalog.MaxListLimit {
		candidateLimit = catalog.MaxListLimit
	}
	return candidateLimit
}
