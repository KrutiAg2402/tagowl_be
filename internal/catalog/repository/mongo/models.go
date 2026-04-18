package mongorepo

import "time"

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
	Items       []orderItemRecord `bson:"items"`
	CreatedAt   time.Time         `bson:"createdAt"`
}

type orderItemRecord struct {
	StickerID string `json:"stickerId" bson:"stickerId"`
	Quantity  int    `json:"quantity" bson:"quantity"`
}

type metricAggregate struct {
	ID          string `bson:"_id"`
	Views7D     int    `bson:"views7D"`
	Favorites7D int    `bson:"favorites7D"`
	Sales7D     int    `bson:"sales7D"`
}
