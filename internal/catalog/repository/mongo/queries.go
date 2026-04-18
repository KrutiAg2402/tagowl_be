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

func (r *Repository) fetchStickers(ctx context.Context, filter bson.M) ([]catalog.Sticker, error) {
	findOptions := options.Find().SetProjection(bson.M{"_id": 0})
	cursor, err := r.stickers.Find(ctx, filter, findOptions)
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
