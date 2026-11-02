package response

// Meta is the pagination block attached to every list response. totalPages is
// deliberately absent; the client computes it from total and pageSize.
type Meta struct {
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
	Total    int64 `json:"total"`
}

// NewMeta builds the meta block from an already defaulted and clamped page.
func NewMeta(page, pageSize int, total int64) Meta {
	return Meta{
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}
}
