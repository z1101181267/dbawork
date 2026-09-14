// Package handler HTTP 处理器。
package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// OK 统一成功响应。
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": data})
}

// Fail 统一失败响应。
func Fail(c *gin.Context, status int, msg string) {
	c.JSON(status, gin.H{"code": 1, "message": msg})
}

// failErr 业务错误（400）。
func failErr(c *gin.Context, err error) {
	Fail(c, http.StatusBadRequest, err.Error())
}

// pathID 解析路径参数 :id。
func pathID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		Fail(c, http.StatusBadRequest, "无效的 ID")
		return 0, false
	}
	return id, true
}
