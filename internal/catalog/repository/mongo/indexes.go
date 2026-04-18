package mongorepo

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (r *Repository) ensureIndexes(ctx context.Context) error {
	models := []mongo.IndexModel{
		{Keys: bson.D{{Key: "id", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "category", Value: 1}}},
		{Keys: bson.D{{Key: "isActive", Value: 1}}},
	}
	if _, err := r.stickers.Indexes().CreateMany(ctx, models); err != nil {
		return fmt.Errorf("create sticker indexes: %w", err)
	}

	if _, err := r.dailyMetrics.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "stickerId", Value: 1}, {Key: "metricDate", Value: 1}},
		Options: options.Index().SetUnique(true),
	}); err != nil {
		return fmt.Errorf("create metrics index: %w", err)
	}

	if _, err := r.viewEvents.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "stickerId", Value: 1}, {Key: "actorKey", Value: 1}, {Key: "viewedOn", Value: 1}},
		Options: options.Index().SetUnique(true),
	}); err != nil {
		return fmt.Errorf("create view events index: %w", err)
	}

	if _, err := r.favorites.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "stickerId", Value: 1}, {Key: "actorKey", Value: 1}},
		Options: options.Index().SetUnique(true),
	}); err != nil {
		return fmt.Errorf("create favorites index: %w", err)
	}

	if _, err := r.orders.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "id", Value: 1}},
		Options: options.Index().SetUnique(true),
	}); err != nil {
		return fmt.Errorf("create order index: %w", err)
	}

	return nil
}
