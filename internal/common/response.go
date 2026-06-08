package common

import "github.com/gin-gonic/gin"

type APIResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type PaginationMeta struct {
	Total      int64 `json:"total"`
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	TotalPages int   `json:"totalPages"`
}

type PaginatedResponse struct {
	Success bool           `json:"success"`
	Message string         `json:"message"`
	Data    any            `json:"data,omitempty"`
	Meta    PaginationMeta `json:"meta"`
}

func Success(c *gin.Context, status int, message string, data any) {
	c.JSON(status, APIResponse{
		Success: true,
		Message: message,
		Data:    data,
	})
}

func Error(c *gin.Context, status int, message string) {
	c.JSON(status, APIResponse{
		Success: false,
		Message: message,
	})
}

func Paginated(c *gin.Context, status int, message string, data any, meta PaginationMeta) {
	c.JSON(status, PaginatedResponse{
		Success: true,
		Message: message,
		Data:    data,
		Meta:    meta,
	})
}
