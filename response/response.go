package response

import (
	"encoding/json"
	"net/http"
)

const (
	CodeOK             = 0
	CodeValidation     = -1
	CodeUnauthorized   = -4
	CodeForbidden      = -3
	CodeNotFound       = -404
	CodeRateLimited    = -429
	CodeInternal       = -500
	CodeNotImplemented = -9000
)

type Envelope struct {
	Code int    `json:"code"`
	Msg  string `json:"msg,omitempty"`
	Data any    `json:"data,omitempty"`
}

type Pagination struct {
	Total      int64 `json:"total"`
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	TotalPages int   `json:"total_pages"`
	Items      any   `json:"items"`
}

func OK(w http.ResponseWriter, data any) {
	JSON(w, http.StatusOK, Envelope{Code: CodeOK, Data: data})
}

func BusinessError(w http.ResponseWriter, code int, msg string, data any) {
	JSON(w, http.StatusOK, Envelope{Code: code, Msg: msg, Data: data})
}

func Error(w http.ResponseWriter, status int, code int, msg string) {
	JSON(w, status, Envelope{Code: code, Msg: msg})
}

func JSON(w http.ResponseWriter, status int, payload Envelope) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func NewPagination(total int64, page, pageSize int, items any) Pagination {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}
	return Pagination{
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
		Items:      items,
	}
}
