package utils

import "github.com/gin-gonic/gin"

type Meta struct {
	Code    int    `json:"code"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type SuccessResponse struct {
	Meta Meta        `json:"meta"`
	Data interface{} `json:"data"`
}

type ErrorResponse struct {
	Meta   Meta        `json:"meta"`
	Errors interface{} `json:"errors,omitempty"`
}

type PaginationMeta struct {
	CurrentPage  int   `json:"current_page"`
	TotalPages   int   `json:"total_pages"`
	TotalRecords int64 `json:"total_records"`
	Limit        int   `json:"limit"`
}

type PaginatedResponse struct {
	Meta       Meta           `json:"meta"`
	Data       interface{}    `json:"data"`
	Pagination PaginationMeta `json:"pagination"`
}

func BuildResponse(code int, status string, message string) Meta {
	return Meta{
		Code:    code,
		Status:  status,
		Message: message,
	}
}

func SuccessResult(c *gin.Context, code int, message string, data interface{}) {
	res := SuccessResponse{
		Meta: BuildResponse(code, "success", message),
		Data: data,
	}
	c.JSON(code, res)
}

func ErrorResult(c *gin.Context, code int, message string, errors interface{}) {
	res := ErrorResponse{
		Meta:   BuildResponse(code, "error", message),
		Errors: errors,
	}
	c.JSON(code, res)
}

func PaginatedResult(c *gin.Context, code int, message string, data interface{}, pagination PaginationMeta) {
	res := PaginatedResponse{
		Meta:       BuildResponse(code, "success", message),
		Data:       data,
		Pagination: pagination,
	}
	c.JSON(code, res)
}
