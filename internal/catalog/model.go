package catalog

import "tagowl/backend/internal/catalog/models"

type Sticker = models.Sticker
type StickerFilter = models.StickerFilter
type ListResponse = models.ListResponse
type AdminListResponse = models.AdminListResponse
type AdminCreateStickerRequest = models.AdminCreateStickerRequest
type AdminUpdateStickerRequest = models.AdminUpdateStickerRequest
type AdminUpdatePriceRequest = models.AdminUpdatePriceRequest
type AdminUpdateStatusRequest = models.AdminUpdateStatusRequest

type Category = models.Category
type CategoryListResponse = models.CategoryListResponse
type AdminCategoryListResponse = models.AdminCategoryListResponse
type AdminCreateCategoryRequest = models.AdminCreateCategoryRequest
type AdminUpdateCategoryRequest = models.AdminUpdateCategoryRequest
type AdminUpdateCategoryStatusRequest = models.AdminUpdateCategoryStatusRequest

type HomeSection = models.HomeSection
type HomeResponse = models.HomeResponse

type EventRequest = models.EventRequest
type EventResponse = models.EventResponse

type OrderItemRequest = models.OrderItemRequest
type OrderCreateRequest = models.OrderCreateRequest
type OrderResponse = models.OrderResponse
