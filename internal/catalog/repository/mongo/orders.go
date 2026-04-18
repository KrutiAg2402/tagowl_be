package mongorepo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"tagowl/backend/internal/catalog"

	"github.com/google/uuid"
)

func (r *Repository) CreateOrder(ctx context.Context, request catalog.OrderCreateRequest) (catalog.OrderResponse, error) {
	items, totalQuantity, err := normalizeOrderItems(request.Items)
	if err != nil {
		return catalog.OrderResponse{}, err
	}

	now := time.Now().UTC()
	day := startOfDayUTC(now)

	stickerIDs := make([]string, 0, len(items))
	for _, item := range items {
		if _, ok, err := r.fetchStickerByID(ctx, item.StickerID, false); err != nil || !ok {
			if err != nil {
				return catalog.OrderResponse{}, err
			}
			return catalog.OrderResponse{}, fmt.Errorf("%w: %s", catalog.ErrStickerNotFound, item.StickerID)
		}
		stickerIDs = append(stickerIDs, item.StickerID)
	}

	orderID := uuid.NewString()
	orderItems := make([]orderItemRecord, 0, len(items))
	for _, item := range items {
		orderItems = append(orderItems, orderItemRecord{StickerID: item.StickerID, Quantity: item.Quantity})
		if err := r.incrementMetrics(ctx, item.StickerID, day, 0, 0, item.Quantity); err != nil {
			return catalog.OrderResponse{}, err
		}
	}

	_, err = r.orders.InsertOne(ctx, orderDocument{
		ID:          orderID,
		CustomerKey: strings.TrimSpace(request.CustomerKey),
		Items:       orderItems,
		CreatedAt:   now,
	})
	if err != nil {
		return catalog.OrderResponse{}, err
	}

	stickers, err := r.fetchStickersByIDs(ctx, stickerIDs, false)
	if err != nil {
		return catalog.OrderResponse{}, err
	}
	stickers, err = r.attachMetrics(ctx, stickers)
	if err != nil {
		return catalog.OrderResponse{}, err
	}

	return catalog.OrderResponse{
		OrderID:       orderID,
		CustomerKey:   strings.TrimSpace(request.CustomerKey),
		ItemCount:     len(items),
		TotalQuantity: totalQuantity,
		CreatedAt:     now,
		Stickers:      stickers,
	}, nil
}
