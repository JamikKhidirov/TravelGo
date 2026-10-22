package model

type PaginationResponse struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	Total      int   `json:"total"`
	TotalPages int   `json:"total_pages"`
}
