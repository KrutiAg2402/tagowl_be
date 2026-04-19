package models

import "time"

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
