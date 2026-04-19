package mongorepo

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"tagowl/backend/internal/catalog"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

const (
	defaultConnectTimeout = 10 * time.Second
	trendingCategoryCap   = 2
	freshnessWindowInDays = 30
)

type Repository struct {
	client       *mongo.Client
	stickers     *mongo.Collection
	categories   *mongo.Collection
	dailyMetrics *mongo.Collection
	viewEvents   *mongo.Collection
	favorites    *mongo.Collection
	orders       *mongo.Collection
}

type catalogFile struct {
	Stickers []catalog.Sticker `json:"stickers"`
}

func New(uri, databaseName, collectionName, seedPath string) (*Repository, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultConnectTimeout)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("connect to mongodb: %w", err)
	}

	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, fmt.Errorf("ping mongodb: %w", err)
	}

	database := client.Database(databaseName)
	repo := &Repository{
		client:       client,
		stickers:     database.Collection(collectionName),
		categories:   database.Collection(collectionName + "_categories"),
		dailyMetrics: database.Collection(collectionName + "_daily_metrics"),
		viewEvents:   database.Collection(collectionName + "_view_events"),
		favorites:    database.Collection(collectionName + "_favorites"),
		orders:       database.Collection(collectionName + "_orders"),
	}

	if err := repo.ensureIndexes(ctx); err != nil {
		_ = repo.Close()
		return nil, err
	}

	if err := repo.seedIfEmpty(ctx, seedPath); err != nil {
		_ = repo.Close()
		return nil, err
	}

	if err := repo.seedCategoriesIfEmpty(ctx); err != nil {
		_ = repo.Close()
		return nil, err
	}

	return repo, nil
}

func (r *Repository) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultConnectTimeout)
	defer cancel()
	return r.client.Disconnect(ctx)
}

func (r *Repository) seedIfEmpty(ctx context.Context, seedPath string) error {
	count, err := r.stickers.CountDocuments(ctx, map[string]any{})
	if err != nil {
		return fmt.Errorf("count stickers: %w", err)
	}
	if count > 0 {
		return nil
	}

	payload, err := os.ReadFile(seedPath)
	if err != nil {
		return fmt.Errorf("read seed file: %w", err)
	}

	var file catalogFile
	if err := json.Unmarshal(payload, &file); err != nil {
		return fmt.Errorf("decode seed file: %w", err)
	}

	if len(file.Stickers) == 0 {
		return fmt.Errorf("seed file contains no stickers")
	}

	now := time.Now().UTC()
	day := startOfDayUTC(now)

	stickerDocs := make([]interface{}, 0, len(file.Stickers))
	metricDocs := make([]interface{}, 0, len(file.Stickers))
	for _, sticker := range file.Stickers {
		if sticker.UpdatedAt.IsZero() {
			sticker.UpdatedAt = sticker.CreatedAt
		}
		sticker.IsActive = true
		sticker.Tags = normalizeTags(sticker.Tags)
		stickerDocs = append(stickerDocs, sticker)

		metricDocs = append(metricDocs, metricDailyDocument{
			StickerID:      sticker.ID,
			MetricDate:     day,
			ViewsCount:     sticker.Views7D,
			FavoritesCount: sticker.Favorites7D,
			SalesCount:     sticker.Sales7D,
		})
	}

	if _, err := r.stickers.InsertMany(ctx, stickerDocs); err != nil {
		return fmt.Errorf("seed stickers: %w", err)
	}
	if _, err := r.dailyMetrics.InsertMany(ctx, metricDocs); err != nil {
		return fmt.Errorf("seed metrics: %w", err)
	}

	return nil
}

func (r *Repository) seedCategoriesIfEmpty(ctx context.Context) error {
	count, err := r.categories.CountDocuments(ctx, map[string]any{})
	if err != nil {
		return fmt.Errorf("count categories: %w", err)
	}
	if count > 0 {
		return nil
	}

	values, err := r.stickers.Distinct(ctx, "category", map[string]any{})
	if err != nil {
		return fmt.Errorf("read sticker categories: %w", err)
	}

	categories := uniqueCategoryNames(values)
	if len(categories) == 0 {
		return nil
	}

	now := time.Now().UTC()
	docs := make([]interface{}, 0, len(categories))
	usedIDs := make(map[string]int, len(categories))
	for index, name := range categories {
		id := categoryIDFromName(name)
		usedIDs[id]++
		if usedIDs[id] > 1 {
			id = fmt.Sprintf("%s-%d", id, usedIDs[id])
		}

		docs = append(docs, catalog.Category{
			ID:             id,
			Name:           name,
			NormalizedName: normalizeCategoryName(name),
			Rank:           index + 1,
			IsActive:       true,
			CreatedAt:      now,
			UpdatedAt:      now,
		})
	}

	if _, err := r.categories.InsertMany(ctx, docs); err != nil {
		return fmt.Errorf("seed categories: %w", err)
	}

	return nil
}
