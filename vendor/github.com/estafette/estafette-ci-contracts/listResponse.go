package contracts

// ListResponse is a container for paginated filtered list items
type ListResponse struct {
	Items      []interface{} `json:"items"`
	Pagination Pagination    `json:"pagination"`
}

// Pagination indicates the current page, the size of the pages and total pages / items
type Pagination struct {
	Page       int `json:"page"`
	Size       int `json:"size"`
	TotalPages int `json:"totalPages"`
	TotalItems int `json:"totalItems"`
}
