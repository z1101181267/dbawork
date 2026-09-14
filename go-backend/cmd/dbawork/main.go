// DBAWORK go-backend 入口：API 网关 + 元数据 CRUD。
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"dbawork/internal/config"
	"dbawork/internal/crypto"
	"dbawork/internal/db"
	"dbawork/internal/grpcclient"
	"dbawork/internal/handler"
	"dbawork/internal/repository"
	"dbawork/internal/router"
	"dbawork/internal/service"
)

func main() {
	cfg := config.Load()
	log.Printf("DBAWORK go-backend 启动: http=%s data=%s py-grpc=%s auth=%v",
		cfg.HTTPAddr, cfg.DataDir, cfg.PyGRPCAddr, cfg.AuthEnabled)

	if err := os.MkdirAll(cfg.DriversDir, 0o755); err != nil {
		log.Fatalf("创建驱动目录失败: %v", err)
	}

	key, err := crypto.LoadOrCreateKey(cfg.SecretKeyPath)
	if err != nil {
		log.Fatalf("加载加密密钥失败: %v", err)
	}
	cipher, err := crypto.New(key)
	if err != nil {
		log.Fatalf("初始化加密器失败: %v", err)
	}

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("打开数据库失败: %v", err)
	}
	defer func() { _ = database.Close() }()
	if err := db.Migrate(database); err != nil {
		log.Fatalf("数据库迁移失败: %v", err)
	}

	groupRepo := repository.NewGroupRepo(database)
	dsRepo := repository.NewDatasourceRepo(database)
	driverRepo := repository.NewDriverRepo(database)
	dtRepo := repository.NewDbTypeRepo(database)

	grpcCli := grpcclient.New(cfg.PyGRPCAddr)
	driverSvc := service.NewDriverService(driverRepo, dtRepo, grpcCli)
	if err := driverSvc.SeedBuiltin(context.Background()); err != nil {
		log.Fatalf("初始化内置驱动失败: %v", err)
	}
	dsSvc := service.NewDatasourceService(dsRepo, groupRepo, dtRepo, cipher, grpcCli)
	groupSvc := service.NewGroupService(groupRepo, dsRepo)

	r := router.New(router.Deps{
		Cfg:        cfg,
		Auth:       handler.NewAuthHandler(cfg),
		Health:     handler.NewHealthHandler(cfg),
		Group:      handler.NewGroupHandler(groupSvc),
		Datasource: handler.NewDatasourceHandler(dsSvc),
		Driver:     handler.NewDriverHandler(driverSvc),
	})

	srv := &http.Server{Addr: cfg.HTTPAddr, Handler: r}
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP 服务异常退出: %v", err)
		}
	}()
	log.Printf("HTTP 服务已就绪: http://127.0.0.1%s/api/v1", cfg.HTTPAddr)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	log.Println("正在关闭……")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	_ = grpcCli.Close()
}
