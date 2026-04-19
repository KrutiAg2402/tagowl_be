package models

import "time"

type Category struct {
	ID             string     `json:"id" bson:"id"`
	Name           string     `json:"name" bson:"name"`
	NormalizedName string     `json:"-" bson:"normalizedName"`
	Description    string     `json:"description,omitempty" bson:"description,omitempty"`
	ImageURL       string     `json:"imageUrl,omitempty" bson:"imageUrl,omitempty"`
	Rank           int        `json:"rank" bson:"rank"`
	IsActive       bool       `json:"isActive" bson:"isActive"`
	CreatedAt      time.Time  `json:"createdAt" bson:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt" bson:"updatedAt"`
	DeletedAt      *time.Time `json:"deletedAt,omitempty" bson:"deletedAt,omitempty"`
}

type CategoryListResponse struct {
	Items []Category `json:"items"`
	Count int        `json:"count"`
}

type AdminCategoryListResponse struct {
	Items           []Category `json:"items"`
	Count           int        `json:"count"`
	IncludeInactive bool       `json:"includeInactive"`
}

type AdminCreateCategoryRequest struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	ImageURL    string `json:"imageUrl"`
	Rank        int    `json:"rank"`
	IsActive    *bool  `json:"isActive"`
}

type AdminUpdateCategoryRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	ImageURL    *string `json:"imageUrl"`
	Rank        *int    `json:"rank"`
	IsActive    *bool   `json:"isActive"`
}

type AdminUpdateCategoryStatusRequest struct {
	IsActive bool `json:"isActive"`
}
