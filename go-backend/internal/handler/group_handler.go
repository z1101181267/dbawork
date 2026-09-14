package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"dbawork/internal/model"
	"dbawork/internal/repository"
	"dbawork/internal/service"
)

// GroupHandler 分组接口。
type GroupHandler struct {
	svc *service.GroupService
}

// NewGroupHandler 构造。
func NewGroupHandler(svc *service.GroupService) *GroupHandler { return &GroupHandler{svc: svc} }

type groupInput struct {
	Name        string `json:"name"`
	ParentID    int64  `json:"parent_id"`
	SortOrder   int    `json:"sort_order"`
	Description string `json:"description"`
}

// List 分组树。
func (h *GroupHandler) List(c *gin.Context) {
	tree, err := h.svc.Tree(c.Request.Context())
	if err != nil {
		Fail(c, http.StatusInternalServerError, "查询分组失败: "+err.Error())
		return
	}
	OK(c, tree)
}

// Create 新建分组。
func (h *GroupHandler) Create(c *gin.Context) {
	var in groupInput
	if err := c.ShouldBindJSON(&in); err != nil {
		Fail(c, http.StatusBadRequest, "请求参数不合法")
		return
	}
	g, err := h.svc.Create(c.Request.Context(), &model.Group{
		Name: in.Name, ParentID: in.ParentID, SortOrder: in.SortOrder, Description: in.Description,
	})
	if err != nil {
		failErr(c, err)
		return
	}
	OK(c, g)
}

// Update 更新分组。
func (h *GroupHandler) Update(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var in groupInput
	if err := c.ShouldBindJSON(&in); err != nil {
		Fail(c, http.StatusBadRequest, "请求参数不合法")
		return
	}
	g, err := h.svc.Update(c.Request.Context(), &model.Group{
		ID: id, Name: in.Name, ParentID: in.ParentID, SortOrder: in.SortOrder, Description: in.Description,
	})
	if err != nil {
		failErr(c, err)
		return
	}
	OK(c, g)
}

// Delete 删除分组。
func (h *GroupHandler) Delete(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	err := h.svc.Delete(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrGroupHasContent) {
			Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		failErr(c, err)
		return
	}
	OK(c, gin.H{})
}
