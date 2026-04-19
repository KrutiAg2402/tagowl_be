package mongorepo

import (
	"context"
	"regexp"
	"strings"
	"time"

	"tagowl/backend/internal/catalog"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (r *Repository) fetchStickers(ctx context.Context, filter bson.M, findOptions ...*options.FindOptions) ([]catalog.Sticker, error) {
	if len(findOptions) == 0 || findOptions[0] == nil {
		findOptions = []*options.FindOptions{options.Find().SetProjection(bson.M{"_id": 0})}
	}

	cursor, err := r.stickers.Find(ctx, filter, findOptions...)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var items []catalog.Sticker
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}

	for index := range items {
		items[index].Tags = normalizeTags(items[index].Tags)
	}

	return items, nil
}

func (r *Repository) fetchStickerByID(ctx context.Context, id string, includeInactive bool) (catalog.Sticker, bool, error) {
	filter := bson.M{"id": id}
	if !includeInactive {
		filter["isActive"] = true
	}

	var sticker catalog.Sticker
	err := r.stickers.FindOne(ctx, filter, options.FindOne().SetProjection(bson.M{"_id": 0})).Decode(&sticker)
	if err == mongo.ErrNoDocuments {
		return catalog.Sticker{}, false, nil
	}
	if err != nil {
		return catalog.Sticker{}, false, err
	}

	sticker.Tags = normalizeTags(sticker.Tags)
	return sticker, true, nil
}

func (r *Repository) fetchStickersByIDs(ctx context.Context, ids []string, includeInactive bool) ([]catalog.Sticker, error) {
	if len(ids) == 0 {
		return []catalog.Sticker{}, nil
	}

	filter := bson.M{"id": bson.M{"$in": ids}}
	if !includeInactive {
		filter["isActive"] = true
	}

	items, err := r.fetchStickers(ctx, filter)
	if err != nil {
		return nil, err
	}

	order := make(map[string]int, len(ids))
	for index, id := range ids {
		order[id] = index
	}
	sortStickersByIDOrder(items, order)
	return items, nil
}

func (r *Repository) fetchPublicStickerIDs(ctx context.Context, filter bson.M, sortKey string, limit int) ([]string, error) {
	if limit <= 0 {
		return []string{}, nil
	}

	now := time.Now().UTC()
	cutoff := startOfDayUTC(now.AddDate(0, 0, -6))
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: filter}},
		r.metricsLookupStage(cutoff),
		{{Key: "$set", Value: bson.M{
			"metricAgg": bson.M{"$ifNull": bson.A{
				bson.M{"$arrayElemAt": bson.A{"$metricAgg", 0}},
				bson.M{},
			}},
		}}},
		{{Key: "$set", Value: bson.M{
			"views7D":       bson.M{"$ifNull": bson.A{"$metricAgg.views7D", 0}},
			"favorites7D":   bson.M{"$ifNull": bson.A{"$metricAgg.favorites7D", 0}},
			"sales7D":       bson.M{"$ifNull": bson.A{"$metricAgg.sales7D", 0}},
			"trendingScore": mongoTrendingScoreExpression(now),
		}}},
		{{Key: "$sort", Value: publicSortDocument(sortKey)}},
		{{Key: "$limit", Value: limit}},
		{{Key: "$project", Value: bson.M{"_id": 0, "id": 1}}},
	}

	cursor, err := r.stickers.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var rows []struct {
		ID string `bson:"id"`
	}
	if err := cursor.All(ctx, &rows); err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	return ids, nil
}

func (r *Repository) fetchOptimizedPublicStickers(ctx context.Context, filter bson.M, sortKey string, limit int) ([]catalog.Sticker, error) {
	ids, err := r.fetchPublicStickerIDs(ctx, filter, sortKey, limit)
	if err != nil {
		return nil, err
	}

	items, err := r.fetchStickersByIDs(ctx, ids, false)
	if err != nil {
		return nil, err
	}

	return r.attachMetrics(ctx, items)
}

func (r *Repository) fetchDistinctPublicStickerCategories(ctx context.Context) ([]string, error) {
	values, err := r.stickers.Distinct(ctx, "category", buildPublicStickerFilter("", ""))
	if err != nil {
		return nil, err
	}
	return uniqueCategoryNames(values), nil
}

func (r *Repository) attachMetrics(ctx context.Context, items []catalog.Sticker) ([]catalog.Sticker, error) {
	if len(items) == 0 {
		return items, nil
	}

	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}

	cutoff := startOfDayUTC(time.Now().UTC().AddDate(0, 0, -6))
	cursor, err := r.dailyMetrics.Aggregate(ctx, mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"stickerId":  bson.M{"$in": ids},
			"metricDate": bson.M{"$gte": cutoff},
		}}},
		{{Key: "$group", Value: bson.M{
			"_id":         "$stickerId",
			"views7D":     bson.M{"$sum": "$viewsCount"},
			"favorites7D": bson.M{"$sum": "$favoritesCount"},
			"sales7D":     bson.M{"$sum": "$salesCount"},
		}}},
	})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	metricMap := make(map[string]metricAggregate, len(ids))
	for cursor.Next(ctx) {
		var metric metricAggregate
		if err := cursor.Decode(&metric); err != nil {
			return nil, err
		}
		metricMap[metric.ID] = metric
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	enriched := cloneStickers(items)
	for index := range enriched {
		if metric, ok := metricMap[enriched[index].ID]; ok {
			enriched[index].Views7D = metric.Views7D
			enriched[index].Favorites7D = metric.Favorites7D
			enriched[index].Sales7D = metric.Sales7D
		}
		enriched[index] = enrichSticker(enriched[index], now)
	}

	return enriched, nil
}

func (r *Repository) metricsLookupStage(cutoff time.Time) bson.D {
	return bson.D{{Key: "$lookup", Value: bson.M{
		"from": r.dailyMetrics.Name(),
		"let":  bson.M{"stickerID": "$id"},
		"pipeline": mongo.Pipeline{
			{{Key: "$match", Value: bson.M{"$expr": bson.M{"$and": bson.A{
				bson.M{"$eq": bson.A{"$stickerId", "$$stickerID"}},
				bson.M{"$gte": bson.A{"$metricDate", cutoff}},
			}}}}},
			{{Key: "$group", Value: bson.M{
				"_id":         "$stickerId",
				"views7D":     bson.M{"$sum": "$viewsCount"},
				"favorites7D": bson.M{"$sum": "$favoritesCount"},
				"sales7D":     bson.M{"$sum": "$salesCount"},
			}}},
		},
		"as": "metricAgg",
	}}}
}

func publicSortDocument(sortKey string) bson.D {
	switch sortKey {
	case "trending":
		return bson.D{
			{Key: "trendingScore", Value: -1},
			{Key: "sales7D", Value: -1},
			{Key: "favorites7D", Value: -1},
			{Key: "views7D", Value: -1},
			{Key: "rank", Value: 1},
		}
	case "newest":
		return bson.D{{Key: "createdAt", Value: -1}, {Key: "trendingScore", Value: -1}}
	case "best_selling":
		return bson.D{{Key: "sales7D", Value: -1}, {Key: "trendingScore", Value: -1}}
	case "price_asc":
		return bson.D{{Key: "price", Value: 1}, {Key: "trendingScore", Value: -1}}
	case "price_desc":
		return bson.D{{Key: "price", Value: -1}, {Key: "trendingScore", Value: -1}}
	case "top_rated":
		return bson.D{{Key: "rating", Value: -1}, {Key: "reviewCount", Value: -1}, {Key: "trendingScore", Value: -1}}
	default:
		return bson.D{{Key: "rank", Value: 1}, {Key: "name", Value: 1}}
	}
}

func mongoTrendingScoreExpression(now time.Time) bson.M {
	ageInDays := bson.M{"$divide": bson.A{
		bson.M{"$subtract": bson.A{now, "$createdAt"}},
		86400000,
	}}
	freshnessBoost := bson.M{"$multiply": bson.A{
		bson.M{"$max": bson.A{
			0,
			bson.M{"$subtract": bson.A{freshnessWindowInDays, ageInDays}},
		}},
		0.6,
	}}
	editorialBoost := bson.M{"$multiply": bson.A{
		bson.M{"$max": bson.A{
			0,
			bson.M{"$subtract": bson.A{10, "$rank"}},
		}},
		2,
	}}
	qualityBoost := bson.M{"$add": bson.A{
		bson.M{"$multiply": bson.A{"$rating", 4}},
		bson.M{"$multiply": bson.A{
			bson.M{"$min": bson.A{"$reviewCount", 200}},
			0.1,
		}},
	}}
	engagementBoost := bson.M{"$add": bson.A{
		bson.M{"$multiply": bson.A{"$sales7D", 10}},
		bson.M{"$multiply": bson.A{"$favorites7D", 3}},
		bson.M{"$multiply": bson.A{"$views7D", 0.12}},
	}}

	return bson.M{"$round": bson.A{
		bson.M{"$add": bson.A{
			engagementBoost,
			qualityBoost,
			freshnessBoost,
			editorialBoost,
		}},
		1,
	}}
}

func (r *Repository) incrementMetrics(ctx context.Context, stickerID string, metricDate time.Time, viewsDelta, favoritesDelta, salesDelta int) error {
	update := bson.M{"$inc": bson.M{}}
	incs := update["$inc"].(bson.M)
	if viewsDelta != 0 {
		incs["viewsCount"] = viewsDelta
	}
	if favoritesDelta != 0 {
		incs["favoritesCount"] = favoritesDelta
	}
	if salesDelta != 0 {
		incs["salesCount"] = salesDelta
	}
	if len(incs) == 0 {
		return nil
	}

	update["$setOnInsert"] = bson.M{
		"stickerId":  stickerID,
		"metricDate": metricDate,
	}

	_, err := r.dailyMetrics.UpdateOne(
		ctx,
		bson.M{"stickerId": stickerID, "metricDate": metricDate},
		update,
		options.Update().SetUpsert(true),
	)
	return err
}

func buildPublicStickerFilter(category, tag string) bson.M {
	filter := bson.M{"isActive": true}
	if category != "" {
		filter["category"] = caseInsensitiveExact(category)
	}
	if tag != "" {
		filter["tags"] = caseInsensitiveExact(tag)
	}
	return filter
}

func caseInsensitiveExact(value string) primitive.Regex {
	return primitive.Regex{
		Pattern: "^" + regexp.QuoteMeta(strings.TrimSpace(value)) + "$",
		Options: "i",
	}
}
