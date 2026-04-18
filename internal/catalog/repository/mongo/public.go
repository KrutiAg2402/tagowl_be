package mongorepo

import (
	"context"
	"errors"

	"tagowl/backend/internal/catalog"
)

func (r *Repository) List(ctx context.Context, filter catalog.StickerFilter) ([]catalog.Sticker, error) {
	normalized := catalog.NormalizeFilter(filter)
	items, err := r.fetchStickers(ctx, buildPublicStickerFilter(normalized.Category, normalized.Tag))
	if err != nil {
		return nil, err
	}

	items, err = r.attachMetrics(ctx, items)
	if err != nil {
		return nil, err
	}

	sortStickers(items, normalized.Sort)
	if len(items) > normalized.Limit {
		items = items[:normalized.Limit]
	}

	return items, nil
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
	items, err := r.fetchStickers(ctx, buildPublicStickerFilter("", ""))
	if err != nil {
		return catalog.HomeResponse{}, err
	}
	if len(items) == 0 {
		return catalog.HomeResponse{}, errors.New("catalog is empty")
	}

	items, err = r.attachMetrics(ctx, items)
	if err != nil {
		return catalog.HomeResponse{}, err
	}

	normalizedLimit := limit
	if normalizedLimit <= 0 {
		normalizedLimit = catalog.DefaultHomeLimit
	}
	if normalizedLimit > catalog.MaxListLimit {
		normalizedLimit = catalog.MaxListLimit
	}

	categorySet := make(map[string]struct{})
	for _, sticker := range items {
		categorySet[sticker.Category] = struct{}{}
	}

	categories := make([]string, 0, len(categorySet))
	for category := range categorySet {
		categories = append(categories, category)
	}
	sortStrings(categories)

	topTrending := cloneStickers(items)
	sortStickers(topTrending, "trending")
	topTrending = limitWithCategoryDiversity(topTrending, normalizedLimit, trendingCategoryCap)

	newArrivals := cloneStickers(items)
	sortStickers(newArrivals, "newest")
	if len(newArrivals) > normalizedLimit {
		newArrivals = newArrivals[:normalizedLimit]
	}

	topRated := cloneStickers(items)
	sortStickers(topRated, "top_rated")
	if len(topRated) > normalizedLimit {
		topRated = topRated[:normalizedLimit]
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
