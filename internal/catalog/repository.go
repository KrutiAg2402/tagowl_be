package catalog

import (
	"context"
	"math"
	"sort"
	"strings"
	"time"
)

const (
	defaultListLimit      = 20
	maxListLimit          = 100
	defaultHomeLimit      = 4
	trendingCategoryCap   = 2
	freshnessWindowInDays = 30
)

type Repository interface {
	List(context.Context, StickerFilter) ([]Sticker, error)
	GetByID(context.Context, string) (Sticker, bool, error)
	Home(context.Context, int) (HomeResponse, error)
	RecordView(context.Context, string, string) (EventResponse, error)
	AddFavorite(context.Context, string, string) (EventResponse, error)
	RemoveFavorite(context.Context, string, string) (EventResponse, error)
	CreateOrder(context.Context, OrderCreateRequest) (OrderResponse, error)
	AdminList(context.Context, bool) ([]Sticker, error)
	AdminGetByID(context.Context, string) (Sticker, bool, error)
	AdminCreateSticker(context.Context, AdminCreateStickerRequest) (Sticker, error)
	AdminUpdateSticker(context.Context, string, AdminUpdateStickerRequest) (Sticker, error)
	AdminUpdatePrice(context.Context, string, AdminUpdatePriceRequest) (Sticker, error)
	AdminUpdateStatus(context.Context, string, AdminUpdateStatusRequest) (Sticker, error)
	AdminDeleteSticker(context.Context, string) (Sticker, error)
	Close() error
}

func normalizeFilter(filter StickerFilter) StickerFilter {
	filter.Category = strings.TrimSpace(filter.Category)
	filter.Tag = strings.TrimSpace(filter.Tag)
	filter.Sort = strings.TrimSpace(filter.Sort)

	if filter.Sort == "" {
		filter.Sort = "trending"
	}

	switch {
	case filter.Limit <= 0:
		filter.Limit = defaultListLimit
	case filter.Limit > maxListLimit:
		filter.Limit = maxListLimit
	}

	return filter
}

func containsTag(tags []string, target string) bool {
	for _, tag := range tags {
		if strings.EqualFold(tag, target) {
			return true
		}
	}
	return false
}

func sortStickers(items []Sticker, sortKey string) {
	switch sortKey {
	case "trending":
		sort.Slice(items, func(i, j int) bool {
			if items[i].TrendingScore == items[j].TrendingScore {
				if items[i].Sales7D == items[j].Sales7D {
					if items[i].Favorites7D == items[j].Favorites7D {
						if items[i].Views7D == items[j].Views7D {
							return items[i].Rank < items[j].Rank
						}
						return items[i].Views7D > items[j].Views7D
					}
					return items[i].Favorites7D > items[j].Favorites7D
				}
				return items[i].Sales7D > items[j].Sales7D
			}
			return items[i].TrendingScore > items[j].TrendingScore
		})
	case "newest":
		sort.Slice(items, func(i, j int) bool {
			if items[i].CreatedAt.Equal(items[j].CreatedAt) {
				return items[i].TrendingScore > items[j].TrendingScore
			}
			return items[i].CreatedAt.After(items[j].CreatedAt)
		})
	case "best_selling":
		sort.Slice(items, func(i, j int) bool {
			if items[i].Sales7D == items[j].Sales7D {
				return items[i].TrendingScore > items[j].TrendingScore
			}
			return items[i].Sales7D > items[j].Sales7D
		})
	case "price_asc":
		sort.Slice(items, func(i, j int) bool {
			if items[i].Price == items[j].Price {
				return items[i].TrendingScore > items[j].TrendingScore
			}
			return items[i].Price < items[j].Price
		})
	case "price_desc":
		sort.Slice(items, func(i, j int) bool {
			if items[i].Price == items[j].Price {
				return items[i].TrendingScore > items[j].TrendingScore
			}
			return items[i].Price > items[j].Price
		})
	case "top_rated":
		sort.Slice(items, func(i, j int) bool {
			if items[i].Rating == items[j].Rating {
				if items[i].ReviewCount == items[j].ReviewCount {
					return items[i].TrendingScore > items[j].TrendingScore
				}
				return items[i].ReviewCount > items[j].ReviewCount
			}
			return items[i].Rating > items[j].Rating
		})
	default:
		sort.Slice(items, func(i, j int) bool {
			if items[i].Rank == items[j].Rank {
				return items[i].Name < items[j].Name
			}
			return items[i].Rank < items[j].Rank
		})
	}
}

func enrichSticker(sticker Sticker, now time.Time) Sticker {
	sticker.TrendingScore = calculateTrendingScore(sticker, now)
	return sticker
}

func calculateTrendingScore(sticker Sticker, now time.Time) float64 {
	ageInDays := now.Sub(sticker.CreatedAt).Hours() / 24
	freshnessBoost := math.Max(0, float64(freshnessWindowInDays)-ageInDays) * 0.6
	editorialBoost := float64(maxInt(0, 10-sticker.Rank)) * 2
	qualityBoost := sticker.Rating*4 + float64(minInt(sticker.ReviewCount, 200))*0.1
	engagementBoost := float64(sticker.Sales7D)*10 +
		float64(sticker.Favorites7D)*3 +
		float64(sticker.Views7D)*0.12

	score := engagementBoost + qualityBoost + freshnessBoost + editorialBoost
	return math.Round(score*10) / 10
}

func limitWithCategoryDiversity(items []Sticker, limit, maxPerCategory int) []Sticker {
	if len(items) <= limit || maxPerCategory <= 0 {
		if len(items) > limit {
			return items[:limit]
		}
		return items
	}

	selected := make([]Sticker, 0, limit)
	categoryCounts := make(map[string]int)
	remaining := make([]Sticker, 0, len(items))

	for _, item := range items {
		if len(selected) == limit {
			break
		}

		if categoryCounts[item.Category] < maxPerCategory {
			selected = append(selected, item)
			categoryCounts[item.Category]++
			continue
		}

		remaining = append(remaining, item)
	}

	for _, item := range remaining {
		if len(selected) == limit {
			break
		}
		selected = append(selected, item)
	}

	return selected
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func cloneStickers(items []Sticker) []Sticker {
	cloned := make([]Sticker, len(items))
	copy(cloned, items)
	return cloned
}
