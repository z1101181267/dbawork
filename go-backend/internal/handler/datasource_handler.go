package handler

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"dbawork/internal/model"
	"dbawork/internal/repository"
	"dbawork/internal/service"
)

// DatasourceHandler 数据源接口。
type DatasourceHandler struct {
	svc *service.DatasourceService
}

// NewDatasourceHandler 构造。
func NewDatasourceHandler(svc *service.DatasourceService) *DatasourceHandler {
	return &DatasourceHandler{svc: svc}
}

// List 数据源列表（支持 group_id/db_type/status/keyword 过滤）。
func (h *DatasourceHandler) List(c *gin.Context) {
	f := repository.DSFilter{DbType: c.Query("db_type"), Keyword: c.Query("keyword")}
	if v := c.Query("group_id"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			f.GroupID = n
		}
	}
	if v := c.Query("status"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			f.Status = n
			f.HasStatus = true
		}
	}
	list, err := h.svc.List(c.Request.Context(), f)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "查询数据源失败: "+err.Error())
		return
	}
	OK(c, list)
}

// Get 数据源详情。
func (h *DatasourceHandler) Get(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	ds, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		failErr(c, err)
		return
	}
	OK(c, ds)
}

// Create 新建数据源。
func (h *DatasourceHandler) Create(c *gin.Context) {
	var in model.DataSourceInput
	if err := c.ShouldBindJSON(&in); err != nil {
		Fail(c, http.StatusBadRequest, "请求参数不合法")
		return
	}
	ds, err := h.svc.Create(c.Request.Context(), &in)
	if err != nil {
		failErr(c, err)
		return
	}
	OK(c, ds)
}

// Update 更新数据源。
func (h *DatasourceHandler) Update(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var in model.DataSourceInput
	if err := c.ShouldBindJSON(&in); err != nil {
		Fail(c, http.StatusBadRequest, "请求参数不合法")
		return
	}
	ds, err := h.svc.Update(c.Request.Context(), id, &in)
	if err != nil {
		failErr(c, err)
		return
	}
	OK(c, ds)
}

// Delete 删除数据源。
func (h *DatasourceHandler) Delete(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		failErr(c, err)
		return
	}
	OK(c, gin.H{})
}

// Test 测试已保存数据源连接。
func (h *DatasourceHandler) Test(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 180*time.Second)
	defer cancel()
	view, err := h.svc.TestByID(ctx, id)
	if err != nil {
		failErr(c, err)
		return
	}
	OK(c, view)
}

// TestAdhoc 未保存前临时测试。
func (h *DatasourceHandler) TestAdhoc(c *gin.Context) {
	var in model.DataSourceInput
	if err := c.ShouldBindJSON(&in); err != nil {
		Fail(c, http.StatusBadRequest, "请求参数不合法")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 180*time.Second)
	defer cancel()
	view, err := h.svc.TestAdhoc(ctx, &in)
	if err != nil {
		failErr(c, err)
		return
	}
	OK(c, view)
}

// Import CSV 批量导入。
func (h *DatasourceHandler) Import(c *gin.Context) {
	fh, err := c.FormFile("file")
	if err != nil {
		Fail(c, http.StatusBadRequest, "缺少上传文件（字段名 file）")
		return
	}
	f, err := fh.Open()
	if err != nil {
		Fail(c, http.StatusBadRequest, "读取文件失败: "+err.Error())
		return
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		Fail(c, http.StatusBadRequest, "读取文件失败: "+err.Error())
		return
	}
	res, err := h.svc.Import(c.Request.Context(), data)
	if err != nil {
		failErr(c, err)
		return
	}
	OK(c, res)
}

// ExportCSV 导出（不含密码）。
func (h *DatasourceHandler) ExportCSV(c *gin.Context) {
	data, err := h.svc.ExportCSV(c.Request.Context())
	if err != nil {
		Fail(c, http.StatusInternalServerError, "导出失败: "+err.Error())
		return
	}
	c.Header("Content-Disposition", `attachment; filename="datasources.csv"`)
	c.Data(http.StatusOK, "text/csv; charset=utf-8", data)
}
