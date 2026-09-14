// Package config 提供运行配置加载（环境变量 + 默认值）。
package config

import (
	"os"
	"path/filepath"
	"strconv"
)

// Config 聚合服务运行所需的全部配置项。
type Config struct {
	HTTPAddr      string // HTTP 监听地址，默认 :8080
	DataDir       string // 数据目录（metadata.db、secret.key、drivers/ 所在）
	DBPath        string // SQLite 文件路径
	SecretKeyPath string // AES-GCM 主密钥文件
	DriversDir    string // 驱动文件目录（JAR 落盘根目录）
	PyGRPCAddr    string // Python gRPC 服务地址

	AuthEnabled  bool // 是否启用 JWT 鉴权（默认关闭，便于本地联调）
	AuthUsername string
	AuthPassword string
	JWTSecret    string
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func getbool(k string, def bool) bool {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

// Load 读取环境变量并生成配置。
//
// 约定：默认数据目录为进程工作目录的上一级 data/（即从 DBAWORK/go-backend 运行时
// 指向 DBAWORK/data），可用 DBAWORK_DATA_DIR 覆盖。
func Load() *Config {
	dataDir := getenv("DBAWORK_DATA_DIR", filepath.Join("..", "data"))
	if abs, err := filepath.Abs(dataDir); err == nil {
		dataDir = abs
	}

	cfg := &Config{
		HTTPAddr:      getenv("DBAWORK_HTTP_ADDR", ":8080"),
		DataDir:       dataDir,
		DBPath:        getenv("DBAWORK_DB_PATH", filepath.Join(dataDir, "metadata.db")),
		SecretKeyPath: getenv("DBAWORK_SECRET_KEY_PATH", filepath.Join(dataDir, "secret.key")),
		DriversDir:    filepath.Join(dataDir, "drivers"),
		PyGRPCAddr:    getenv("DBAWORK_PY_GRPC", "127.0.0.1:50051"),

		AuthEnabled:  getbool("DBAWORK_AUTH_ENABLED", false),
		AuthUsername: getenv("DBAWORK_AUTH_USER", "admin"),
		AuthPassword: getenv("DBAWORK_AUTH_PASS", "admin"),
		JWTSecret:    getenv("DBAWORK_JWT_SECRET", "dbawork-dev-secret-change-me"),
	}
	return cfg
}
