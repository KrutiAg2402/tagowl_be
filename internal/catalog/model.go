package catalog

import "time"

type Sticker struct {
	ID            string     `json:"id" bson:"id"`
	Name          string     `json:"name" bson:"name"`
	Description   string     `json:"description,omitempty" bson:"description,omitempty"`
	ImageURL      string     `json:"imageUrl" bson:"imageUrl"`
	Category      string     `json:"category" bson:"category"`
	Tags          []string   `json:"tags" bson:"tags"`
	Price         float64    `json:"price" bson:"price"`
	Currency      string     `json:"currency" bson:"currency"`
	Rank          int        `json:"rank" bson:"rank"`
	Rating        float64    `json:"rating" bson:"rating"`
	ReviewCount   int        `json:"reviewCount" bson:"reviewCount"`
	IsNewArrival  bool       `json:"isNewArrival" bson:"isNewArrival"`
	IsActive      bool       `json:"isActive" bson:"isActive"`
	Views7D       int        `json:"views7D" bson:"-"`
	Sales7D       int        `json:"sales7D" bson:"-"`
	Favorites7D   int        `json:"favorites7D" bson:"-"`
	TrendingScore float64    `json:"trendingScore" bson:"-"`
	CreatedAt     time.Time  `json:"createdAt" bson:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt" bson:"updatedAt"`
	DeletedAt     *time.Time `json:"deletedAt,omitempty" bson:"deletedAt,omitempty"`
}

type HomeSection struct {
	Key      string    `json:"key"`
	Title    string    `json:"title"`
	Stickers []Sticker `json:"stickers"`
}

type HomeResponse struct {
	Categories []string      `json:"categories"`
	Sections   []HomeSection `json:"sections"`
}

type StickerFilter struct {
	Category string `json:"category"`
	Tag      string `json:"tag"`
	Sort     string `json:"sort"`
	Limit    int    `json:"limit"`
}

type ListResponse struct {
	Items   []Sticker     `json:"items"`
	Count   int           `json:"count"`
	Filters StickerFilter `json:"filters"`
}

type EventRequest struct {
	ActorKey string `json:"actorKey"`
}

type EventResponse struct {
	Action   string  `json:"action"`
	Recorded bool    `json:"recorded"`
	Sticker  Sticker `json:"sticker"`
}

type OrderItemRequest struct {
	StickerID string `json:"stickerId"`
	Quantity  int    `json:"quantity"`
}

type OrderCreateRequest struct {
	CustomerKey string             `json:"customerKey"`
	Items       []OrderItemRequest `json:"items"`
}

type OrderResponse struct {
	OrderID       string    `json:"orderId"`
	CustomerKey   string    `json:"customerKey"`
	ItemCount     int       `json:"itemCount"`
	TotalQuantity int       `json:"totalQuantity"`
	CreatedAt     time.Time `json:"createdAt"`
	Stickers      []Sticker `json:"stickers"`
}

type AdminListResponse struct {
	Items           []Sticker `json:"items"`
	Count           int       `json:"count"`
	IncludeInactive bool      `json:"includeInactive"`
}

type AdminCreateStickerRequest struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	ImageURL     string   `json:"imageUrl"`
	Category     string   `json:"category"`
	Tags         []string `json:"tags"`
	Price        float64  `json:"price"`
	Currency     string   `json:"currency"`
	Rank         int      `json:"rank"`
	Rating       float64  `json:"rating"`
	ReviewCount  int      `json:"reviewCount"`
	IsNewArrival bool     `json:"isNewArrival"`
	IsActive     *bool    `json:"isActive"`
}

type AdminUpdateStickerRequest struct {
	Name         *string   `json:"name"`
	Description  *string   `json:"description"`
	ImageURL     *string   `json:"imageUrl"`
	Category     *string   `json:"category"`
	Tags         *[]string `json:"tags"`
	Price        *float64  `json:"price"`
	Currency     *string   `json:"currency"`
	Rank         *int      `json:"rank"`
	Rating       *float64  `json:"rating"`
	ReviewCount  *int      `json:"reviewCount"`
	IsNewArrival *bool     `json:"isNewArrival"`
	IsActive     *bool     `json:"isActive"`
}

type AdminUpdatePriceRequest struct {
	Price    float64 `json:"price"`
	Currency string  `json:"currency"`
}

type AdminUpdateStatusRequest struct {
	IsActive bool `json:"isActive"`
}
