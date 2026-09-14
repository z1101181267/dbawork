// Package router 注册 HTTP 路由（全部 /api/v1 前缀）。
package router

import (
	"github.com/gin-gonic/gin"

	"dbawork/internal/config"
	"dbawork/internal/handler"
	"dbawork/internal/middleware"
)

// Deps 路由依赖集合。
type Deps struct {
	Cfg        *config.Config
	Auth       *handler.AuthHandler
	Health     *handler.HealthHandler
	Group      *handler.GroupHandler
	Datasource *handler.DatasourceHandler
	Driver     *handler.DriverHandler
}

// New 构建 Gin 引擎。
func New(d Deps) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery(), middleware.CORS())
	r.MaxMultipartMemory = 64 << 20

	api := r.Group("/api/v1")
	api.GET("/health", d.Health.Health)
	api.POST("/auth/login", d.Auth.Login)

	authed := api.Group("")
	authed.Use(middleware.Auth(d.Cfg))

	// 分组
	authed.GET("/groups", d.Group.List)
	authed.POST("/groups", d.Group.Create)
	authed.PUT("/groups/:id", d.Group.Update)
	authed.DELETE("/groups/:id", d.Group.Delete)

	// 数据源
	authed.GET("/datasources", d.Datasource.List)
	authed.POST("/datasources", d.Datasource.Create)
	authed.GET("/datasources/export.csv", d.Datasource.ExportCSV)
	authed.POST("/datasources/import", d.Datasource.Import)
	authed.POST("/datasources/test-adhoc", d.Datasource.TestAdhoc)
	authed.GET("/datasources/:id", d.Datasource.Get)
	authed.PUT("/datasources/:id", d.Datasource.Update)
	authed.DELETE("/datasources/:id", d.Datasource.Delete)
	authed.POST("/datasources/:id/test", d.Datasource.Test)

	// 类型目录
	authed.GET("/db-types", d.Driver.ListDbTypes)
	authed.POST("/db-types", d.Driver.CreateDbType)
	authed.DELETE("/db-types/:key", d.Driver.DeleteDbType)
	authed.POST("/db-types/:key/restore", d.Driver.RestoreDbType)

	// 驱动管理
	authed.GET("/drivers", d.Driver.List)
	authed.POST("/drivers/jdbc", d.Driver.UploadJar)
	authed.POST("/drivers/detect", d.Driver.DetectAll)
	authed.POST("/drivers/detect/:db_type", d.Driver.DetectOne)
	authed.POST("/drivers/install", d.Driver.Install)
	authed.POST("/drivers/uninstall", d.Driver.Uninstall)
	authed.DELETE("/drivers/:id", d.Driver.Delete)
	authed.POST("/drivers/:id/activate", d.Driver.Activate)

	return r
}
