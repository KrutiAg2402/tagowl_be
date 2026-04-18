package mongorepo

import (
	"context"
	"errors"
	"strings"
	"time"

	"tagowl/backend/internal/catalog"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (r *Repository) RecordView(ctx context.Context, stickerID, actorKey string) (catalog.EventResponse, error) {
	if _, ok, err := r.fetchStickerByID(ctx, stickerID, false); err != nil || !ok {
		if err != nil {
			return catalog.EventResponse{}, err
		}
		return catalog.EventResponse{}, catalog.ErrStickerNotFound
	}

	now := time.Now().UTC()
	day := startOfDayUTC(now)
	recorded := false

	if strings.TrimSpace(actorKey) == "" {
		if err := r.incrementMetrics(ctx, stickerID, day, 1, 0, 0); err != nil {
			return catalog.EventResponse{}, err
		}
		recorded = true
	} else {
		result, err := r.viewEvents.UpdateOne(
			ctx,
			bson.M{"stickerId": stickerID, "actorKey": actorKey, "viewedOn": day},
			bson.M{"$setOnInsert": viewEventDocument{
				StickerID: stickerID,
				ActorKey:  actorKey,
				ViewedOn:  day,
				CreatedAt: now,
			}},
			options.Update().SetUpsert(true),
		)
		if err != nil {
			return catalog.EventResponse{}, err
		}

		recorded = result.UpsertedCount > 0
		if recorded {
			if err := r.incrementMetrics(ctx, stickerID, day, 1, 0, 0); err != nil {
				return catalog.EventResponse{}, err
			}
		}
	}

	sticker, ok, err := r.GetByID(ctx, stickerID)
	if err != nil || !ok {
		if err != nil {
			return catalog.EventResponse{}, err
		}
		return catalog.EventResponse{}, catalog.ErrStickerNotFound
	}

	action := "view_recorded"
	if !recorded {
		action = "view_already_recorded_today"
	}

	return catalog.EventResponse{Action: action, Recorded: recorded, Sticker: sticker}, nil
}

func (r *Repository) AddFavorite(ctx context.Context, stickerID, actorKey string) (catalog.EventResponse, error) {
	actorKey = strings.TrimSpace(actorKey)
	if actorKey == "" {
		return catalog.EventResponse{}, catalog.ErrActorKeyRequired
	}
	if _, ok, err := r.fetchStickerByID(ctx, stickerID, false); err != nil || !ok {
		if err != nil {
			return catalog.EventResponse{}, err
		}
		return catalog.EventResponse{}, catalog.ErrStickerNotFound
	}

	now := time.Now().UTC()
	day := startOfDayUTC(now)
	recorded := false

	var favorite favoriteDocument
	err := r.favorites.FindOne(ctx, bson.M{"stickerId": stickerID, "actorKey": actorKey}).Decode(&favorite)
	switch {
	case errors.Is(err, mongo.ErrNoDocuments):
		_, err = r.favorites.InsertOne(ctx, favoriteDocument{
			StickerID: stickerID,
			ActorKey:  actorKey,
			IsActive:  true,
			CreatedAt: now,
			UpdatedAt: now,
		})
		if err != nil {
			return catalog.EventResponse{}, err
		}
		recorded = true
	case err != nil:
		return catalog.EventResponse{}, err
	case !favorite.IsActive:
		_, err = r.favorites.UpdateOne(
			ctx,
			bson.M{"stickerId": stickerID, "actorKey": actorKey},
			bson.M{"$set": bson.M{"isActive": true, "updatedAt": now}},
		)
		if err != nil {
			return catalog.EventResponse{}, err
		}
		recorded = true
	}

	if recorded {
		if err := r.incrementMetrics(ctx, stickerID, day, 0, 1, 0); err != nil {
			return catalog.EventResponse{}, err
		}
	}

	sticker, ok, err := r.GetByID(ctx, stickerID)
	if err != nil || !ok {
		if err != nil {
			return catalog.EventResponse{}, err
		}
		return catalog.EventResponse{}, catalog.ErrStickerNotFound
	}

	action := "favorite_created"
	if !recorded {
		action = "favorite_already_active"
	}

	return catalog.EventResponse{Action: action, Recorded: recorded, Sticker: sticker}, nil
}

func (r *Repository) RemoveFavorite(ctx context.Context, stickerID, actorKey string) (catalog.EventResponse, error) {
	actorKey = strings.TrimSpace(actorKey)
	if actorKey == "" {
		return catalog.EventResponse{}, catalog.ErrActorKeyRequired
	}

	result, err := r.favorites.UpdateOne(
		ctx,
		bson.M{"stickerId": stickerID, "actorKey": actorKey, "isActive": true},
		bson.M{"$set": bson.M{"isActive": false, "updatedAt": time.Now().UTC()}},
	)
	if err != nil {
		return catalog.EventResponse{}, err
	}

	sticker, ok, err := r.AdminGetByID(ctx, stickerID)
	if err != nil || !ok {
		if err != nil {
			return catalog.EventResponse{}, err
		}
		return catalog.EventResponse{}, catalog.ErrStickerNotFound
	}
	sticker = enrichSticker(sticker, time.Now().UTC())

	recorded := result.ModifiedCount > 0
	action := "favorite_removed"
	if !recorded {
		action = "favorite_not_active"
	}

	return catalog.EventResponse{Action: action, Recorded: recorded, Sticker: sticker}, nil
}
