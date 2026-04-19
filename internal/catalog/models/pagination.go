package models

type Pagination struct {
	Page   int `json:"page"`
	Limit  int `json:"limit"`
	Offset int `json:"-"`
}

type PaginationResponse struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Count      int   `json:"count"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"totalPages"`
}
