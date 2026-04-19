package mongorepo

import (
	"context"
	"strings"
	"time"

	"tagowl/backend/internal/catalog"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (r *Repository) AdminList(ctx context.Context, includeInactive bool, pagination catalog.Pagination) ([]catalog.Sticker, int64, error) {
	filter := bson.M{}
	if !includeInactive {
		filter["isActive"] = true
	}

	total, err := r.stickers.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	findOptions := options.Find().
		SetProjection(bson.M{"_id": 0}).
		SetSort(bson.D{{Key: "updatedAt", Value: -1}}).
		SetSkip(int64(pagination.Offset)).
		SetLimit(int64(pagination.Limit))

	items, err := r.fetchStickers(ctx, filter, findOptions)
	if err != nil {
		return nil, 0, err
	}
	items, err = r.attachMetrics(ctx, items)
	if err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r *Repository) AdminGetByID(ctx context.Context, id string) (catalog.Sticker, bool, error) {
	sticker, ok, err := r.fetchStickerByID(ctx, id, true)
	if err != nil || !ok {
		return catalog.Sticker{}, ok, err
	}

	items, err := r.attachMetrics(ctx, []catalog.Sticker{sticker})
	if err != nil {
		return catalog.Sticker{}, false, err
	}
	return items[0], true, nil
}

func (r *Repository) AdminCreateSticker(ctx context.Context, request catalog.AdminCreateStickerRequest) (catalog.Sticker, error) {
	now := time.Now().UTC()
	sticker, err := buildStickerFromCreateRequest(request, now)
	if err != nil {
		return catalog.Sticker{}, err
	}

	_, err = r.stickers.InsertOne(ctx, sticker)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return catalog.Sticker{}, catalog.ErrDuplicateSticker
		}
		return catalog.Sticker{}, err
	}

	created, ok, err := r.AdminGetByID(ctx, sticker.ID)
	if err != nil || !ok {
		if err != nil {
			return catalog.Sticker{}, err
		}
		return catalog.Sticker{}, catalog.ErrStickerNotFound
	}
	return created, nil
}

func (r *Repository) AdminUpdateSticker(ctx context.Context, id string, request catalog.AdminUpdateStickerRequest) (catalog.Sticker, error) {
	update, unset, err := buildStickerPatch(request, time.Now().UTC())
	if err != nil {
		return catalog.Sticker{}, err
	}

	updateDoc := bson.M{}
	if len(update) > 0 {
		updateDoc["$set"] = update
	}
	if len(unset) > 0 {
		updateDoc["$unset"] = unset
	}
	if len(updateDoc) == 0 {
		return catalog.Sticker{}, catalog.ErrNoStickerChanges
	}

	result, err := r.stickers.UpdateOne(ctx, bson.M{"id": id}, updateDoc)
	if err != nil {
		return catalog.Sticker{}, err
	}
	if result.MatchedCount == 0 {
		return catalog.Sticker{}, catalog.ErrStickerNotFound
	}

	sticker, ok, err := r.AdminGetByID(ctx, id)
	if err != nil || !ok {
		if err != nil {
			return catalog.Sticker{}, err
		}
		return catalog.Sticker{}, catalog.ErrStickerNotFound
	}
	return sticker, nil
}

func (r *Repository) AdminUpdatePrice(ctx context.Context, id string, request catalog.AdminUpdatePriceRequest) (catalog.Sticker, error) {
	if request.Price < 0 {
		return catalog.Sticker{}, catalog.ErrInvalidPrice
	}

	update := bson.M{
		"price":     request.Price,
		"updatedAt": time.Now().UTC(),
	}
	if strings.TrimSpace(request.Currency) != "" {
		update["currency"] = strings.TrimSpace(request.Currency)
	}

	result, err := r.stickers.UpdateOne(ctx, bson.M{"id": id}, bson.M{"$set": update})
	if err != nil {
		return catalog.Sticker{}, err
	}
	if result.MatchedCount == 0 {
		return catalog.Sticker{}, catalog.ErrStickerNotFound
	}

	sticker, ok, err := r.AdminGetByID(ctx, id)
	if err != nil || !ok {
		if err != nil {
			return catalog.Sticker{}, err
		}
		return catalog.Sticker{}, catalog.ErrStickerNotFound
	}
	return sticker, nil
}

func (r *Repository) AdminUpdateStatus(ctx context.Context, id string, request catalog.AdminUpdateStatusRequest) (catalog.Sticker, error) {
	now := time.Now().UTC()
	update := bson.M{
		"isActive":  request.IsActive,
		"updatedAt": now,
	}
	updateDoc := bson.M{"$set": update}
	if request.IsActive {
		updateDoc["$unset"] = bson.M{"deletedAt": ""}
	} else {
		update["deletedAt"] = now
	}

	result, err := r.stickers.UpdateOne(ctx, bson.M{"id": id}, updateDoc)
	if err != nil {
		return catalog.Sticker{}, err
	}
	if result.MatchedCount == 0 {
		return catalog.Sticker{}, catalog.ErrStickerNotFound
	}

	sticker, ok, err := r.AdminGetByID(ctx, id)
	if err != nil || !ok {
		if err != nil {
			return catalog.Sticker{}, err
		}
		return catalog.Sticker{}, catalog.ErrStickerNotFound
	}
	return sticker, nil
}

func (r *Repository) AdminDeleteSticker(ctx context.Context, id string) (catalog.Sticker, error) {
	return r.AdminUpdateStatus(ctx, id, catalog.AdminUpdateStatusRequest{IsActive: false})
}
