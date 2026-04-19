package catalog

import (
	"context"
	"strings"
)

const (
	DefaultListLimit  = 20
	MaxListLimit      = 100
	DefaultHomeLimit  = 4
	DefaultAdminPage  = 1
	DefaultAdminLimit = 20
	MaxAdminLimit     = 100
)

type Repository interface {
	ListCategories(context.Context) ([]Category, error)
	List(context.Context, StickerFilter) ([]Sticker, error)
	GetByID(context.Context, string) (Sticker, bool, error)
	Home(context.Context, int) (HomeResponse, error)
	RecordView(context.Context, string, string) (EventResponse, error)
	AddFavorite(context.Context, string, string) (EventResponse, error)
	RemoveFavorite(context.Context, string, string) (EventResponse, error)
	CreateOrder(context.Context, OrderCreateRequest) (OrderResponse, error)
	AdminListCategories(context.Context, bool, Pagination) ([]Category, int64, error)
	AdminGetCategoryByID(context.Context, string) (Category, bool, error)
	AdminCreateCategory(context.Context, AdminCreateCategoryRequest) (Category, error)
	AdminUpdateCategory(context.Context, string, AdminUpdateCategoryRequest) (Category, error)
	AdminUpdateCategoryStatus(context.Context, string, AdminUpdateCategoryStatusRequest) (Category, error)
	AdminDeleteCategory(context.Context, string) (Category, error)
	AdminList(context.Context, bool, Pagination) ([]Sticker, int64, error)
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

func NormalizePagination(page, limit int) Pagination {
	if page <= 0 {
		page = DefaultAdminPage
	}

	switch {
	case limit <= 0:
		limit = DefaultAdminLimit
	case limit > MaxAdminLimit:
		limit = MaxAdminLimit
	}

	return Pagination{
		Page:   page,
		Limit:  limit,
		Offset: (page - 1) * limit,
	}
}

func NewPaginationResponse(pagination Pagination, count int, total int64) PaginationResponse {
	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(pagination.Limit) - 1) / int64(pagination.Limit))
	}

	return PaginationResponse{
		Page:       pagination.Page,
		Limit:      pagination.Limit,
		Count:      count,
		Total:      total,
		TotalPages: totalPages,
	}
}
