package catalog

import "errors"

var (
	ErrStickerNotFound   = errors.New("sticker not found")
	ErrCategoryNotFound  = errors.New("category not found")
	ErrActorKeyRequired  = errors.New("actor key is required")
	ErrEmptyOrder        = errors.New("order requires at least one item")
	ErrInvalidSticker    = errors.New("sticker payload is invalid")
	ErrInvalidCategory   = errors.New("category payload is invalid")
	ErrInvalidPrice      = errors.New("price must be greater than or equal to zero")
	ErrDuplicateSticker  = errors.New("sticker id already exists")
	ErrDuplicateCategory = errors.New("category already exists")
	ErrNoStickerChanges  = errors.New("no sticker fields were provided to update")
	ErrNoCategoryChanges = errors.New("no category fields were provided to update")
)
