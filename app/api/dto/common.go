package dto

// PagedResponse -- ответ с пагинацией.
type PagedResponse[T any] struct {
	Items      []T   `json:"items"`
	TotalCount int64 `json:"totalCount"`
	Page       int   `json:"page"`
	Size       int   `json:"size"`
}

// ProblemDetails -- стандартный формат ошибки RFC 7807.
type ProblemDetails struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Status int    `json:"status"`
	Detail string `json:"detail,omitempty"`
}
