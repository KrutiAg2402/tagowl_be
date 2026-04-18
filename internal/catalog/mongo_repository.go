package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

var (
	ErrStickerNotFound   = errors.New("sticker not found")
	ErrActorKeyRequired  = errors.New("actor key is required")
	ErrEmptyOrder        = errors.New("order requires at least one item")
	ErrInvalidSticker    = errors.New("sticker payload is invalid")
	ErrInvalidPrice      = errors.New("price must be greater than or equal to zero")
	ErrDuplicateSticker  = errors.New("sticker id already exists")
	ErrNoStickerChanges  = errors.New("no sticker fields were provided to update")
	defaultConnectTimout = 10 * time.Second
)

type MongoRepository struct {
	client       *mongo.Client
	stickers     *mongo.Collection
	dailyMetrics *mongo.Collection
	viewEvents   *mongo.Collection
	favorites    *mongo.Collection
	orders       *mongo.Collection
}

type catalogFile struct {
	Stickers []Sticker `json:"stickers"`
}

type metricDailyDocument struct {
	StickerID      string    `bson:"stickerId"`
	MetricDate     time.Time `bson:"metricDate"`
	ViewsCount     int       `bson:"viewsCount"`
	FavoritesCount int       `bson:"favoritesCount"`
	SalesCount     int       `bson:"salesCount"`
}

type favoriteDocument struct {
	StickerID string    `bson:"stickerId"`
	ActorKey  string    `bson:"actorKey"`
	IsActive  bool      `bson:"isActive"`
	CreatedAt time.Time `bson:"createdAt"`
	UpdatedAt time.Time `bson:"updatedAt"`
}

type viewEventDocument struct {
	StickerID string    `bson:"stickerId"`
	ActorKey  string    `bson:"actorKey"`
	ViewedOn  time.Time `bson:"viewedOn"`
	CreatedAt time.Time `bson:"createdAt"`
}

type orderDocument struct {
	ID          string            `bson:"id"`
	CustomerKey string            `bson:"customerKey,omitempty"`
	Items       []OrderItemRecord `bson:"items"`
	CreatedAt   time.Time         `bson:"createdAt"`
}

type OrderItemRecord struct {
	StickerID string `json:"stickerId" bson:"stickerId"`
	Quantity  int    `json:"quantity" bson:"quantity"`
}

type metricAggregate struct {
	ID          string `bson:"_id"`
	Views7D     int    `bson:"views7D"`
	Favorites7D int    `bson:"favorites7D"`
	Sales7D     int    `bson:"sales7D"`
}

func NewMongoRepository(uri, databaseName, collectionName, seedPath string) (*MongoRepository, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultConnectTimout)
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
	repo := &MongoRepository{
		client:       client,
		stickers:     database.Collection(collectionName),
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

	return repo, nil
}

func (r *MongoRepository) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultConnectTimout)
	defer cancel()
	return r.client.Disconnect(ctx)
}

func (r *MongoRepository) List(ctx context.Context, filter StickerFilter) ([]Sticker, error) {
	normalized := normalizeFilter(filter)
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

func (r *MongoRepository) GetByID(ctx context.Context, id string) (Sticker, bool, error) {
	sticker, ok, err := r.fetchStickerByID(ctx, id, false)
	if err != nil || !ok {
		return Sticker{}, ok, err
	}

	items, err := r.attachMetrics(ctx, []Sticker{sticker})
	if err != nil {
		return Sticker{}, false, err
	}
	return items[0], true, nil
}

func (r *MongoRepository) Home(ctx context.Context, limit int) (HomeResponse, error) {
	items, err := r.fetchStickers(ctx, buildPublicStickerFilter("", ""))
	if err != nil {
		return HomeResponse{}, err
	}
	if len(items) == 0 {
		return HomeResponse{}, errors.New("catalog is empty")
	}

	items, err = r.attachMetrics(ctx, items)
	if err != nil {
		return HomeResponse{}, err
	}

	normalizedLimit := limit
	if normalizedLimit <= 0 {
		normalizedLimit = defaultHomeLimit
	}
	if normalizedLimit > maxListLimit {
		normalizedLimit = maxListLimit
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

	return HomeResponse{
		Categories: categories,
		Sections: []HomeSection{
			{Key: "top-trending", Title: "Top Trending", Stickers: topTrending},
			{Key: "new-arrivals", Title: "New Arrivals", Stickers: newArrivals},
			{Key: "top-rated", Title: "Top Rated", Stickers: topRated},
		},
	}, nil
}

func (r *MongoRepository) RecordView(ctx context.Context, stickerID, actorKey string) (EventResponse, error) {
	if _, ok, err := r.fetchStickerByID(ctx, stickerID, false); err != nil || !ok {
		if err != nil {
			return EventResponse{}, err
		}
		return EventResponse{}, ErrStickerNotFound
	}

	now := time.Now().UTC()
	day := startOfDayUTC(now)
	recorded := false

	if strings.TrimSpace(actorKey) == "" {
		if err := r.incrementMetrics(ctx, stickerID, day, 1, 0, 0); err != nil {
			return EventResponse{}, err
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
			return EventResponse{}, err
		}

		recorded = result.UpsertedCount > 0
		if recorded {
			if err := r.incrementMetrics(ctx, stickerID, day, 1, 0, 0); err != nil {
				return EventResponse{}, err
			}
		}
	}

	sticker, ok, err := r.GetByID(ctx, stickerID)
	if err != nil || !ok {
		if err != nil {
			return EventResponse{}, err
		}
		return EventResponse{}, ErrStickerNotFound
	}

	action := "view_recorded"
	if !recorded {
		action = "view_already_recorded_today"
	}

	return EventResponse{Action: action, Recorded: recorded, Sticker: sticker}, nil
}

func (r *MongoRepository) AddFavorite(ctx context.Context, stickerID, actorKey string) (EventResponse, error) {
	actorKey = strings.TrimSpace(actorKey)
	if actorKey == "" {
		return EventResponse{}, ErrActorKeyRequired
	}
	if _, ok, err := r.fetchStickerByID(ctx, stickerID, false); err != nil || !ok {
		if err != nil {
			return EventResponse{}, err
		}
		return EventResponse{}, ErrStickerNotFound
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
			return EventResponse{}, err
		}
		recorded = true
	case err != nil:
		return EventResponse{}, err
	case !favorite.IsActive:
		_, err = r.favorites.UpdateOne(
			ctx,
			bson.M{"stickerId": stickerID, "actorKey": actorKey},
			bson.M{"$set": bson.M{"isActive": true, "updatedAt": now}},
		)
		if err != nil {
			return EventResponse{}, err
		}
		recorded = true
	}

	if recorded {
		if err := r.incrementMetrics(ctx, stickerID, day, 0, 1, 0); err != nil {
			return EventResponse{}, err
		}
	}

	sticker, ok, err := r.GetByID(ctx, stickerID)
	if err != nil || !ok {
		if err != nil {
			return EventResponse{}, err
		}
		return EventResponse{}, ErrStickerNotFound
	}

	action := "favorite_created"
	if !recorded {
		action = "favorite_already_active"
	}

	return EventResponse{Action: action, Recorded: recorded, Sticker: sticker}, nil
}

func (r *MongoRepository) RemoveFavorite(ctx context.Context, stickerID, actorKey string) (EventResponse, error) {
	actorKey = strings.TrimSpace(actorKey)
	if actorKey == "" {
		return EventResponse{}, ErrActorKeyRequired
	}

	result, err := r.favorites.UpdateOne(
		ctx,
		bson.M{"stickerId": stickerID, "actorKey": actorKey, "isActive": true},
		bson.M{"$set": bson.M{"isActive": false, "updatedAt": time.Now().UTC()}},
	)
	if err != nil {
		return EventResponse{}, err
	}

	sticker, ok, err := r.AdminGetByID(ctx, stickerID)
	if err != nil || !ok {
		if err != nil {
			return EventResponse{}, err
		}
		return EventResponse{}, ErrStickerNotFound
	}
	sticker = enrichSticker(sticker, time.Now().UTC())

	recorded := result.ModifiedCount > 0
	action := "favorite_removed"
	if !recorded {
		action = "favorite_not_active"
	}

	return EventResponse{Action: action, Recorded: recorded, Sticker: sticker}, nil
}

func (r *MongoRepository) CreateOrder(ctx context.Context, request OrderCreateRequest) (OrderResponse, error) {
	items, totalQuantity, err := normalizeOrderItems(request.Items)
	if err != nil {
		return OrderResponse{}, err
	}

	now := time.Now().UTC()
	day := startOfDayUTC(now)

	stickerIDs := make([]string, 0, len(items))
	for _, item := range items {
		if _, ok, err := r.fetchStickerByID(ctx, item.StickerID, false); err != nil || !ok {
			if err != nil {
				return OrderResponse{}, err
			}
			return OrderResponse{}, fmt.Errorf("%w: %s", ErrStickerNotFound, item.StickerID)
		}
		stickerIDs = append(stickerIDs, item.StickerID)
	}

	orderID := uuid.NewString()
	orderItems := make([]OrderItemRecord, 0, len(items))
	for _, item := range items {
		orderItems = append(orderItems, OrderItemRecord{StickerID: item.StickerID, Quantity: item.Quantity})
		if err := r.incrementMetrics(ctx, item.StickerID, day, 0, 0, item.Quantity); err != nil {
			return OrderResponse{}, err
		}
	}

	_, err = r.orders.InsertOne(ctx, orderDocument{
		ID:          orderID,
		CustomerKey: strings.TrimSpace(request.CustomerKey),
		Items:       orderItems,
		CreatedAt:   now,
	})
	if err != nil {
		return OrderResponse{}, err
	}

	stickers, err := r.fetchStickersByIDs(ctx, stickerIDs, false)
	if err != nil {
		return OrderResponse{}, err
	}
	stickers, err = r.attachMetrics(ctx, stickers)
	if err != nil {
		return OrderResponse{}, err
	}

	return OrderResponse{
		OrderID:       orderID,
		CustomerKey:   strings.TrimSpace(request.CustomerKey),
		ItemCount:     len(items),
		TotalQuantity: totalQuantity,
		CreatedAt:     now,
		Stickers:      stickers,
	}, nil
}

func (r *MongoRepository) AdminList(ctx context.Context, includeInactive bool) ([]Sticker, error) {
	filter := bson.M{}
	if !includeInactive {
		filter["isActive"] = true
	}

	items, err := r.fetchStickers(ctx, filter)
	if err != nil {
		return nil, err
	}
	items, err = r.attachMetrics(ctx, items)
	if err != nil {
		return nil, err
	}
	sortAdminStickers(items)
	return items, nil
}

func (r *MongoRepository) AdminGetByID(ctx context.Context, id string) (Sticker, bool, error) {
	sticker, ok, err := r.fetchStickerByID(ctx, id, true)
	if err != nil || !ok {
		return Sticker{}, ok, err
	}

	items, err := r.attachMetrics(ctx, []Sticker{sticker})
	if err != nil {
		return Sticker{}, false, err
	}
	return items[0], true, nil
}

func (r *MongoRepository) AdminCreateSticker(ctx context.Context, request AdminCreateStickerRequest) (Sticker, error) {
	now := time.Now().UTC()
	sticker, err := buildStickerFromCreateRequest(request, now)
	if err != nil {
		return Sticker{}, err
	}

	_, err = r.stickers.InsertOne(ctx, sticker)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return Sticker{}, ErrDuplicateSticker
		}
		return Sticker{}, err
	}

	created, ok, err := r.AdminGetByID(ctx, sticker.ID)
	if err != nil || !ok {
		if err != nil {
			return Sticker{}, err
		}
		return Sticker{}, ErrStickerNotFound
	}
	return created, nil
}

func (r *MongoRepository) AdminUpdateSticker(ctx context.Context, id string, request AdminUpdateStickerRequest) (Sticker, error) {
	update, unset, err := buildStickerPatch(request, time.Now().UTC())
	if err != nil {
		return Sticker{}, err
	}

	updateDoc := bson.M{}
	if len(update) > 0 {
		updateDoc["$set"] = update
	}
	if len(unset) > 0 {
		updateDoc["$unset"] = unset
	}
	if len(updateDoc) == 0 {
		return Sticker{}, ErrNoStickerChanges
	}

	result, err := r.stickers.UpdateOne(ctx, bson.M{"id": id}, updateDoc)
	if err != nil {
		return Sticker{}, err
	}
	if result.MatchedCount == 0 {
		return Sticker{}, ErrStickerNotFound
	}

	sticker, ok, err := r.AdminGetByID(ctx, id)
	if err != nil || !ok {
		if err != nil {
			return Sticker{}, err
		}
		return Sticker{}, ErrStickerNotFound
	}
	return sticker, nil
}

func (r *MongoRepository) AdminUpdatePrice(ctx context.Context, id string, request AdminUpdatePriceRequest) (Sticker, error) {
	if request.Price < 0 {
		return Sticker{}, ErrInvalidPrice
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
		return Sticker{}, err
	}
	if result.MatchedCount == 0 {
		return Sticker{}, ErrStickerNotFound
	}

	sticker, ok, err := r.AdminGetByID(ctx, id)
	if err != nil || !ok {
		if err != nil {
			return Sticker{}, err
		}
		return Sticker{}, ErrStickerNotFound
	}
	return sticker, nil
}

func (r *MongoRepository) AdminUpdateStatus(ctx context.Context, id string, request AdminUpdateStatusRequest) (Sticker, error) {
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
		return Sticker{}, err
	}
	if result.MatchedCount == 0 {
		return Sticker{}, ErrStickerNotFound
	}

	sticker, ok, err := r.AdminGetByID(ctx, id)
	if err != nil || !ok {
		if err != nil {
			return Sticker{}, err
		}
		return Sticker{}, ErrStickerNotFound
	}
	return sticker, nil
}

func (r *MongoRepository) AdminDeleteSticker(ctx context.Context, id string) (Sticker, error) {
	return r.AdminUpdateStatus(ctx, id, AdminUpdateStatusRequest{IsActive: false})
}

func (r *MongoRepository) ensureIndexes(ctx context.Context) error {
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

func (r *MongoRepository) seedIfEmpty(ctx context.Context, seedPath string) error {
	count, err := r.stickers.CountDocuments(ctx, bson.M{})
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
		return errors.New("seed file contains no stickers")
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

func (r *MongoRepository) fetchStickers(ctx context.Context, filter bson.M) ([]Sticker, error) {
	options := options.Find().SetProjection(bson.M{"_id": 0})
	cursor, err := r.stickers.Find(ctx, filter, options)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var items []Sticker
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}

	for index := range items {
		items[index].Tags = normalizeTags(items[index].Tags)
	}

	return items, nil
}

func (r *MongoRepository) fetchStickerByID(ctx context.Context, id string, includeInactive bool) (Sticker, bool, error) {
	filter := bson.M{"id": id}
	if !includeInactive {
		filter["isActive"] = true
	}

	var sticker Sticker
	err := r.stickers.FindOne(ctx, filter, options.FindOne().SetProjection(bson.M{"_id": 0})).Decode(&sticker)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Sticker{}, false, nil
	}
	if err != nil {
		return Sticker{}, false, err
	}

	sticker.Tags = normalizeTags(sticker.Tags)
	return sticker, true, nil
}

func (r *MongoRepository) fetchStickersByIDs(ctx context.Context, ids []string, includeInactive bool) ([]Sticker, error) {
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

func (r *MongoRepository) attachMetrics(ctx context.Context, items []Sticker) ([]Sticker, error) {
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

func (r *MongoRepository) incrementMetrics(ctx context.Context, stickerID string, metricDate time.Time, viewsDelta, favoritesDelta, salesDelta int) error {
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

func buildStickerFromCreateRequest(request AdminCreateStickerRequest, now time.Time) (Sticker, error) {
	name := strings.TrimSpace(request.Name)
	imageURL := strings.TrimSpace(request.ImageURL)
	category := strings.TrimSpace(request.Category)
	if name == "" || imageURL == "" || category == "" {
		return Sticker{}, ErrInvalidSticker
	}
	if request.Price < 0 {
		return Sticker{}, ErrInvalidPrice
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

	sticker := Sticker{
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

func buildStickerPatch(request AdminUpdateStickerRequest, now time.Time) (bson.M, bson.M, error) {
	update := bson.M{"updatedAt": now}
	unset := bson.M{}
	changed := false

	if request.Name != nil {
		name := strings.TrimSpace(*request.Name)
		if name == "" {
			return nil, nil, ErrInvalidSticker
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
			return nil, nil, ErrInvalidSticker
		}
		update["imageUrl"] = imageURL
		changed = true
	}
	if request.Category != nil {
		category := strings.TrimSpace(*request.Category)
		if category == "" {
			return nil, nil, ErrInvalidSticker
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
			return nil, nil, ErrInvalidPrice
		}
		update["price"] = *request.Price
		changed = true
	}
	if request.Currency != nil {
		currency := strings.TrimSpace(*request.Currency)
		if currency == "" {
			return nil, nil, ErrInvalidSticker
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

func normalizeOrderItems(items []OrderItemRequest) ([]OrderItemRequest, int, error) {
	if len(items) == 0 {
		return nil, 0, ErrEmptyOrder
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

	normalized := make([]OrderItemRequest, 0, len(merged))
	for stickerID, quantity := range merged {
		normalized = append(normalized, OrderItemRequest{StickerID: stickerID, Quantity: quantity})
	}
	sortOrderItems(normalized)
	return normalized, totalQuantity, nil
}

func sortOrderItems(items []OrderItemRequest) {
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

func caseInsensitiveExact(value string) primitive.Regex {
	return primitive.Regex{
		Pattern: "^" + regexp.QuoteMeta(strings.TrimSpace(value)) + "$",
		Options: "i",
	}
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

func sortStickersByIDOrder(items []Sticker, order map[string]int) {
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

func sortAdminStickers(items []Sticker) {
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

func startOfDayUTC(value time.Time) time.Time {
	value = value.UTC()
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}
