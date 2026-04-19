package mongorepo

import (
	"errors"
	"math"
	"sort"
	"strings"
	"time"

	"tagowl/backend/internal/catalog"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
)

func buildStickerFromCreateRequest(request catalog.AdminCreateStickerRequest, now time.Time) (catalog.Sticker, error) {
	name := strings.TrimSpace(request.Name)
	imageURL := strings.TrimSpace(request.ImageURL)
	category := strings.TrimSpace(request.Category)
	if name == "" || imageURL == "" || category == "" {
		return catalog.Sticker{}, catalog.ErrInvalidSticker
	}
	if request.Price < 0 {
		return catalog.Sticker{}, catalog.ErrInvalidPrice
	}

	id := strings.TrimSpace(request.ID)
	if id == "" {
		id = "stk_" + strings.ReplaceAll(uuid.NewString()[:8], "-", "")
	}

	currency := strings.TrimSpace(request.Currency)
	if currency == "" {
		currency = "USD"
	}

	isActive := true
	if request.IsActive != nil {
		isActive = *request.IsActive
	}

	sticker := catalog.Sticker{
		ID:           id,
		Name:         name,
		Description:  strings.TrimSpace(request.Description),
		ImageURL:     imageURL,
		Category:     category,
		Tags:         normalizeTags(request.Tags),
		Price:        request.Price,
		Currency:     currency,
		Rank:         request.Rank,
		Rating:       request.Rating,
		ReviewCount:  request.ReviewCount,
		IsNewArrival: request.IsNewArrival,
		IsActive:     isActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if !isActive {
		deletedAt := now
		sticker.DeletedAt = &deletedAt
	}
	return sticker, nil
}

func buildCategoryFromCreateRequest(request catalog.AdminCreateCategoryRequest, now time.Time) (catalog.Category, error) {
	name := strings.TrimSpace(request.Name)
	if name == "" {
		return catalog.Category{}, catalog.ErrInvalidCategory
	}

	id := strings.TrimSpace(request.ID)
	if id == "" {
		id = categoryIDFromName(name)
	}

	isActive := true
	if request.IsActive != nil {
		isActive = *request.IsActive
	}

	category := catalog.Category{
		ID:             id,
		Name:           name,
		NormalizedName: normalizeCategoryName(name),
		Description:    strings.TrimSpace(request.Description),
		ImageURL:       strings.TrimSpace(request.ImageURL),
		Rank:           request.Rank,
		IsActive:       isActive,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if !isActive {
		deletedAt := now
		category.DeletedAt = &deletedAt
	}

	return category, nil
}

func buildStickerPatch(request catalog.AdminUpdateStickerRequest, now time.Time) (bson.M, bson.M, error) {
	update := bson.M{"updatedAt": now}
	unset := bson.M{}
	changed := false

	if request.Name != nil {
		name := strings.TrimSpace(*request.Name)
		if name == "" {
			return nil, nil, catalog.ErrInvalidSticker
		}
		update["name"] = name
		changed = true
	}
	if request.Description != nil {
		update["description"] = strings.TrimSpace(*request.Description)
		changed = true
	}
	if request.ImageURL != nil {
		imageURL := strings.TrimSpace(*request.ImageURL)
		if imageURL == "" {
			return nil, nil, catalog.ErrInvalidSticker
		}
		update["imageUrl"] = imageURL
		changed = true
	}
	if request.Category != nil {
		category := strings.TrimSpace(*request.Category)
		if category == "" {
			return nil, nil, catalog.ErrInvalidSticker
		}
		update["category"] = category
		changed = true
	}
	if request.Tags != nil {
		update["tags"] = normalizeTags(*request.Tags)
		changed = true
	}
	if request.Price != nil {
		if *request.Price < 0 {
			return nil, nil, catalog.ErrInvalidPrice
		}
		update["price"] = *request.Price
		changed = true
	}
	if request.Currency != nil {
		currency := strings.TrimSpace(*request.Currency)
		if currency == "" {
			return nil, nil, catalog.ErrInvalidSticker
		}
		update["currency"] = currency
		changed = true
	}
	if request.Rank != nil {
		update["rank"] = *request.Rank
		changed = true
	}
	if request.Rating != nil {
		update["rating"] = *request.Rating
		changed = true
	}
	if request.ReviewCount != nil {
		update["reviewCount"] = *request.ReviewCount
		changed = true
	}
	if request.IsNewArrival != nil {
		update["isNewArrival"] = *request.IsNewArrival
		changed = true
	}
	if request.IsActive != nil {
		update["isActive"] = *request.IsActive
		if *request.IsActive {
			unset["deletedAt"] = ""
		} else {
			update["deletedAt"] = now
		}
		changed = true
	}

	if !changed {
		delete(update, "updatedAt")
	}
	return update, unset, nil
}

func buildCategoryPatch(request catalog.AdminUpdateCategoryRequest, now time.Time) (bson.M, bson.M, error) {
	update := bson.M{"updatedAt": now}
	unset := bson.M{}
	changed := false

	if request.Name != nil {
		name := strings.TrimSpace(*request.Name)
		if name == "" {
			return nil, nil, catalog.ErrInvalidCategory
		}
		update["name"] = name
		update["normalizedName"] = normalizeCategoryName(name)
		changed = true
	}
	if request.Description != nil {
		update["description"] = strings.TrimSpace(*request.Description)
		changed = true
	}
	if request.ImageURL != nil {
		update["imageUrl"] = strings.TrimSpace(*request.ImageURL)
		changed = true
	}
	if request.Rank != nil {
		update["rank"] = *request.Rank
		changed = true
	}
	if request.IsActive != nil {
		update["isActive"] = *request.IsActive
		if *request.IsActive {
			unset["deletedAt"] = ""
		} else {
			update["deletedAt"] = now
		}
		changed = true
	}

	if !changed {
		delete(update, "updatedAt")
	}
	return update, unset, nil
}

func normalizeOrderItems(items []catalog.OrderItemRequest) ([]catalog.OrderItemRequest, int, error) {
	if len(items) == 0 {
		return nil, 0, catalog.ErrEmptyOrder
	}

	merged := make(map[string]int)
	totalQuantity := 0
	for _, item := range items {
		stickerID := strings.TrimSpace(item.StickerID)
		if stickerID == "" {
			return nil, 0, errors.New("order item requires stickerId")
		}
		if item.Quantity <= 0 {
			return nil, 0, errors.New("order item quantity must be greater than zero")
		}
		merged[stickerID] += item.Quantity
		totalQuantity += item.Quantity
	}

	normalized := make([]catalog.OrderItemRequest, 0, len(merged))
	for stickerID, quantity := range merged {
		normalized = append(normalized, catalog.OrderItemRequest{StickerID: stickerID, Quantity: quantity})
	}
	sortOrderItems(normalized)
	return normalized, totalQuantity, nil
}

func sortOrderItems(items []catalog.OrderItemRequest) {
	if len(items) < 2 {
		return
	}
	for i := 0; i < len(items)-1; i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].StickerID < items[i].StickerID {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}

func normalizeTags(tags []string) []string {
	if len(tags) == 0 {
		return []string{}
	}

	seen := make(map[string]struct{}, len(tags))
	normalized := make([]string, 0, len(tags))
	for _, tag := range tags {
		cleaned := strings.TrimSpace(tag)
		if cleaned == "" {
			continue
		}
		key := strings.ToLower(cleaned)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, cleaned)
	}
	sortStrings(normalized)
	return normalized
}

func uniqueCategoryNames(values []interface{}) []string {
	seen := make(map[string]struct{}, len(values))
	categories := make([]string, 0, len(values))
	for _, value := range values {
		name, ok := value.(string)
		if !ok {
			continue
		}
		name = strings.TrimSpace(name)
		normalized := normalizeCategoryName(name)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		categories = append(categories, name)
	}
	sortStrings(categories)
	return categories
}

func normalizeCategoryName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func categoryIDFromName(name string) string {
	normalized := normalizeCategoryName(name)

	var builder strings.Builder
	lastDash := false
	for _, value := range normalized {
		isLetter := value >= 'a' && value <= 'z'
		isDigit := value >= '0' && value <= '9'
		if isLetter || isDigit {
			builder.WriteRune(value)
			lastDash = false
			continue
		}
		if !lastDash && builder.Len() > 0 {
			builder.WriteRune('-')
			lastDash = true
		}
	}

	slug := strings.Trim(builder.String(), "-")
	if slug == "" {
		slug = strings.ReplaceAll(uuid.NewString()[:8], "-", "")
	}
	return "cat_" + slug
}

func sortStrings(items []string) {
	if len(items) < 2 {
		return
	}
	for i := 0; i < len(items)-1; i++ {
		for j := i + 1; j < len(items); j++ {
			if strings.ToLower(items[j]) < strings.ToLower(items[i]) {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}

func sortStickersByIDOrder(items []catalog.Sticker, order map[string]int) {
	if len(items) < 2 {
		return
	}
	for i := 0; i < len(items)-1; i++ {
		for j := i + 1; j < len(items); j++ {
			if order[items[j].ID] < order[items[i].ID] {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}

func sortAdminStickers(items []catalog.Sticker) {
	if len(items) < 2 {
		return
	}
	for i := 0; i < len(items)-1; i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].UpdatedAt.After(items[i].UpdatedAt) {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}

func sortCategories(items []catalog.Category) {
	if len(items) < 2 {
		return
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Rank == items[j].Rank {
			return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
		}
		return items[i].Rank < items[j].Rank
	})
}

func startOfDayUTC(value time.Time) time.Time {
	value = value.UTC()
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}

func sortStickers(items []catalog.Sticker, sortKey string) {
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

func enrichSticker(sticker catalog.Sticker, now time.Time) catalog.Sticker {
	sticker.TrendingScore = calculateTrendingScore(sticker, now)
	return sticker
}

func calculateTrendingScore(sticker catalog.Sticker, now time.Time) float64 {
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

func limitWithCategoryDiversity(items []catalog.Sticker, limit, maxPerCategory int) []catalog.Sticker {
	if len(items) <= limit || maxPerCategory <= 0 {
		if len(items) > limit {
			return items[:limit]
		}
		return items
	}

	selected := make([]catalog.Sticker, 0, limit)
	categoryCounts := make(map[string]int)
	remaining := make([]catalog.Sticker, 0, len(items))

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

func cloneStickers(items []catalog.Sticker) []catalog.Sticker {
	cloned := make([]catalog.Sticker, len(items))
	copy(cloned, items)
	return cloned
}
