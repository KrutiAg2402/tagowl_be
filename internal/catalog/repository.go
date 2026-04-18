package catalog

import (
	"context"
	"strings"
)

const (
	DefaultListLimit      = 20
	MaxListLimit          = 100
	DefaultHomeLimit      = 4
)

type Repository interface {
	List(context.Context, StickerFilter) ([]Sticker, error)
	GetByID(context.Context, string) (Sticker, bool, error)
	Home(context.Context, int) (HomeResponse, error)
	RecordView(context.Context, string, string) (EventResponse, error)
	AddFavorite(context.Context, string, string) (EventResponse, error)
	RemoveFavorite(context.Context, string, string) (EventResponse, error)
	CreateOrder(context.Context, OrderCreateRequest) (OrderResponse, error)
	AdminList(context.Context, bool) ([]Sticker, error)
	AdminGetByID(context.Context, string) (Sticker, bool, error)
	AdminCreateSticker(context.Context, AdminCreateStickerRequest) (Sticker, error)
	AdminUpdateSticker(context.Context, string, AdminUpdateStickerRequest) (Sticker, error)
	AdminUpdatePrice(context.Context, string, AdminUpdatePriceRequest) (Sticker, error)
	AdminUpdateStatus(context.Context, string, AdminUpdateStatusRequest) (Sticker, error)
	AdminDeleteSticker(context.Context, string) (Sticker, error)
	Close() error
}

func NormalizeFilter(filter StickerFilter) StickerFilter {
	filter.Category = strings.TrimSpace(filter.Category)
	filter.Tag = strings.TrimSpace(filter.Tag)
	filter.Sort = strings.TrimSpace(filter.Sort)

	if filter.Sort == "" {
		filter.Sort = "trending"
	}

	switch {
	case filter.Limit <= 0:
		filter.Limit = DefaultListLimit
	case filter.Limit > MaxListLimit:
		filter.Limit = MaxListLimit
	}

	return filter
}
