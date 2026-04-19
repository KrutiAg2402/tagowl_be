package mongorepo

import (
	"context"
	"time"

	"tagowl/backend/internal/catalog"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (r *Repository) ListCategories(ctx context.Context) ([]catalog.Category, error) {
	items, err := r.fetchCategories(ctx, false)
	if err != nil {
		return nil, err
	}
	sortCategories(items)
	return items, nil
}

func (r *Repository) AdminListCategories(ctx context.Context, includeInactive bool, pagination catalog.Pagination) ([]catalog.Category, int64, error) {
	filter := buildCategoryFilter(includeInactive)

	total, err := r.categories.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	findOptions := options.Find().
		SetProjection(bson.M{"_id": 0}).
		SetSort(bson.D{{Key: "rank", Value: 1}, {Key: "name", Value: 1}}).
		SetSkip(int64(pagination.Offset)).
		SetLimit(int64(pagination.Limit))

	items, err := r.fetchCategoriesByFilter(ctx, filter, findOptions)
	if err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r *Repository) AdminGetCategoryByID(ctx context.Context, id string) (catalog.Category, bool, error) {
	return r.fetchCategoryByID(ctx, id, true)
}

func (r *Repository) AdminCreateCategory(ctx context.Context, request catalog.AdminCreateCategoryRequest) (catalog.Category, error) {
	now := time.Now().UTC()
	category, err := buildCategoryFromCreateRequest(request, now)
	if err != nil {
		return catalog.Category{}, err
	}

	_, err = r.categories.InsertOne(ctx, category)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return catalog.Category{}, catalog.ErrDuplicateCategory
		}
		return catalog.Category{}, err
	}

	created, ok, err := r.AdminGetCategoryByID(ctx, category.ID)
	if err != nil || !ok {
		if err != nil {
			return catalog.Category{}, err
		}
		return catalog.Category{}, catalog.ErrCategoryNotFound
	}
	return created, nil
}

func (r *Repository) AdminUpdateCategory(ctx context.Context, id string, request catalog.AdminUpdateCategoryRequest) (catalog.Category, error) {
	current, ok, err := r.AdminGetCategoryByID(ctx, id)
	if err != nil || !ok {
		if err != nil {
			return catalog.Category{}, err
		}
		return catalog.Category{}, catalog.ErrCategoryNotFound
	}

	now := time.Now().UTC()
	update, unset, err := buildCategoryPatch(request, now)
	if err != nil {
		return catalog.Category{}, err
	}

	updateDoc := bson.M{}
	if len(update) > 0 {
		updateDoc["$set"] = update
	}
	if len(unset) > 0 {
		updateDoc["$unset"] = unset
	}
	if len(updateDoc) == 0 {
		return catalog.Category{}, catalog.ErrNoCategoryChanges
	}

	result, err := r.categories.UpdateOne(ctx, bson.M{"id": id}, updateDoc)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return catalog.Category{}, catalog.ErrDuplicateCategory
		}
		return catalog.Category{}, err
	}
	if result.MatchedCount == 0 {
		return catalog.Category{}, catalog.ErrCategoryNotFound
	}

	if name, ok := update["name"].(string); ok && normalizeCategoryName(name) != current.NormalizedName {
		if err := r.renameStickerCategory(ctx, current.Name, name, now); err != nil {
			return catalog.Category{}, err
		}
	}

	updated, ok, err := r.AdminGetCategoryByID(ctx, id)
	if err != nil || !ok {
		if err != nil {
			return catalog.Category{}, err
		}
		return catalog.Category{}, catalog.ErrCategoryNotFound
	}
	return updated, nil
}

func (r *Repository) AdminUpdateCategoryStatus(ctx context.Context, id string, request catalog.AdminUpdateCategoryStatusRequest) (catalog.Category, error) {
	isActive := request.IsActive
	return r.AdminUpdateCategory(ctx, id, catalog.AdminUpdateCategoryRequest{IsActive: &isActive})
}

func (r *Repository) AdminDeleteCategory(ctx context.Context, id string) (catalog.Category, error) {
	return r.AdminUpdateCategoryStatus(ctx, id, catalog.AdminUpdateCategoryStatusRequest{IsActive: false})
}

func (r *Repository) fetchCategories(ctx context.Context, includeInactive bool) ([]catalog.Category, error) {
	filter := buildCategoryFilter(includeInactive)
	findOptions := options.Find().
		SetProjection(bson.M{"_id": 0}).
		SetSort(bson.D{{Key: "rank", Value: 1}, {Key: "name", Value: 1}})

	return r.fetchCategoriesByFilter(ctx, filter, findOptions)
}

func (r *Repository) fetchCategoriesByFilter(ctx context.Context, filter bson.M, findOptions ...*options.FindOptions) ([]catalog.Category, error) {
	if len(findOptions) == 0 || findOptions[0] == nil {
		findOptions = []*options.FindOptions{options.Find().SetProjection(bson.M{"_id": 0})}
	}

	cursor, err := r.categories.Find(ctx, filter, findOptions...)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var items []catalog.Category
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}

	return items, nil
}

func buildCategoryFilter(includeInactive bool) bson.M {
	filter := bson.M{}
	if !includeInactive {
		filter["isActive"] = true
	}
	return filter
}

func (r *Repository) fetchCategoryByID(ctx context.Context, id string, includeInactive bool) (catalog.Category, bool, error) {
	filter := bson.M{"id": id}
	if !includeInactive {
		filter["isActive"] = true
	}

	var category catalog.Category
	err := r.categories.FindOne(ctx, filter, options.FindOne().SetProjection(bson.M{"_id": 0})).Decode(&category)
	if err == mongo.ErrNoDocuments {
		return catalog.Category{}, false, nil
	}
	if err != nil {
		return catalog.Category{}, false, err
	}

	return category, true, nil
}

func (r *Repository) renameStickerCategory(ctx context.Context, oldName, newName string, now time.Time) error {
	_, err := r.stickers.UpdateMany(
		ctx,
		bson.M{"category": caseInsensitiveExact(oldName)},
		bson.M{"$set": bson.M{"category": newName, "updatedAt": now}},
	)
	return err
}
