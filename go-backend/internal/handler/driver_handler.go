package handler

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"dbawork/internal/model"
	"dbawork/internal/service"
)

// DriverHandler 驱动管理接口。
type DriverHandler struct {
	svc *service.DriverService
}

// NewDriverHandler 构造。
func NewDriverHandler(svc *service.DriverService) *DriverHandler { return &DriverHandler{svc: svc} }

// List 驱动列表 ?db_type=。
func (h *DriverHandler) List(c *gin.Context) {
	list, err := h.svc.ListDrivers(c.Request.Context(), c.Query("db_type"))
	if err != nil {
		Fail(c, http.StatusInternalServerError, "查询驱动失败: "+err.Error())
		return
	}
	OK(c, list)
}

// DetectAll 全量检测。
func (h *DriverHandler) DetectAll(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 300*time.Second)
	defer cancel()
	res, err := h.svc.DetectAll(ctx)
	if err != nil {
		Fail(c, http.StatusBadGateway, err.Error())
		return
	}
	OK(c, res)
}

// DetectOne 单类型检测。
func (h *DriverHandler) DetectOne(c *gin.Context) {
	dbType := c.Param("db_type")
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()
	rows, err := h.svc.DetectOne(ctx, dbType)
	if err != nil {
		Fail(c, http.StatusBadGateway, err.Error())
		return
	}
	if len(rows) == 0 {
		OK(c, nil)
		return
	}
	OK(c, rows[0])
}

// UploadJar 上传 JAR（multipart: file/db_type/version/driver_class）。
func (h *DriverHandler) UploadJar(c *gin.Context) {
	dbType := strings.TrimSpace(c.PostForm("db_type"))
	version := strings.TrimSpace(c.PostForm("version"))
	driverClass := strings.TrimSpace(c.PostForm("driver_class"))
	fh, err := c.FormFile("file")
	if err != nil {
		Fail(c, http.StatusBadRequest, "缺少 JAR 文件（字段名 file）")
		return
	}
	if !strings.HasSuffix(strings.ToLower(fh.Filename), ".jar") {
		Fail(c, http.StatusBadRequest, "仅支持 .jar 文件")
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
	ctx, cancel := context.WithTimeout(c.Request.Context(), 300*time.Second)
	defer cancel()
	d, err := h.svc.UploadJar(ctx, dbType, version, driverClass, filepath.Base(fh.Filename), data)
	if err != nil {
		failErr(c, err)
		return
	}
	OK(c, d)
}

// Delete 删除驱动。
func (h *DriverHandler) Delete(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()
	if err := h.svc.Delete(ctx, id); err != nil {
		failErr(c, err)
		return
	}
	OK(c, gin.H{})
}

// Activate 设置默认驱动。
func (h *DriverHandler) Activate(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	d, syncMsg, err := h.svc.Activate(c.Request.Context(), id)
	if err != nil {
		failErr(c, err)
		return
	}
	OK(c, gin.H{"driver": d, "sync": syncMsg})
}

// Install 安装 python 驱动。
func (h *DriverHandler) Install(c *gin.Context) {
	var in struct {
		DbType      string `json:"db_type"`
		DriverKind  string `json:"driver_kind"`
		PackageName string `json:"package_name"`
		Version     string `json:"version"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		Fail(c, http.StatusBadRequest, "请求参数不合法")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 600*time.Second)
	defer cancel()
	ok, msg, err := h.svc.Install(ctx, in.DbType, in.DriverKind, in.PackageName, in.Version)
	if err != nil {
		Fail(c, http.StatusBadGateway, err.Error())
		return
	}
	OK(c, gin.H{"ok": ok, "message": msg})
}

// Uninstall 卸载 python 驱动。
func (h *DriverHandler) Uninstall(c *gin.Context) {
	var in struct {
		DbType     string `json:"db_type"`
		DriverKind string `json:"driver_kind"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		Fail(c, http.StatusBadRequest, "请求参数不合法")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 300*time.Second)
	defer cancel()
	ok, msg, err := h.svc.Uninstall(ctx, in.DbType, in.DriverKind)
	if err != nil {
		Fail(c, http.StatusBadGateway, err.Error())
		return
	}
	OK(c, gin.H{"ok": ok, "message": msg})
}

// ListDbTypes 类型目录（合并视图）。
func (h *DriverHandler) ListDbTypes(c *gin.Context) {
	includeHidden := c.Query("include_hidden") == "1" || c.Query("include_hidden") == "true"
	types, err := h.svc.ListDbTypes(c.Request.Context(), includeHidden)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "查询类型目录失败: "+err.Error())
		return
	}
	OK(c, types)
}

// CreateDbType 新建自定义类型。
func (h *DriverHandler) CreateDbType(c *gin.Context) {
	var in struct {
		Key             string `json:"key"`
		NameZh          string `json:"name_zh"`
		NameEn          string `json:"name_en"`
		DriverClassHint string `json:"driver_class_hint"`
		IsJdbc          bool   `json:"is_jdbc"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		Fail(c, http.StatusBadRequest, "请求参数不合法")
		return
	}
	isJdbc := 1
	if !in.IsJdbc {
		isJdbc = 0
	}
	err := h.svc.CreateCustomType(c.Request.Context(), &model.DbTypeCustom{
		DbType: in.Key, NameZh: in.NameZh, NameEn: in.NameEn,
		DriverClassHint: in.DriverClassHint, IsJdbc: isJdbc,
	})
	if err != nil {
		failErr(c, err)
		return
	}
	OK(c, gin.H{})
}

// DeleteDbType 隐藏内置类型 / 删除自定义类型。
func (h *DriverHandler) DeleteDbType(c *gin.Context) {
	key := c.Param("key")
	if err := h.svc.HideOrDeleteType(c.Request.Context(), key); err != nil {
		failErr(c, err)
		return
	}
	OK(c, gin.H{})
}

// RestoreDbType 恢复隐藏类型。
func (h *DriverHandler) RestoreDbType(c *gin.Context) {
	key := c.Param("key")
	if err := h.svc.RestoreType(c.Request.Context(), key); err != nil {
		failErr(c, err)
		return
	}
	OK(c, gin.H{})
}
