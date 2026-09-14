// Package db 负责 SQLite 连接与迁移。
package db

import (
	"database/sql"
	"embed"
	"fmt"
	"path/filepath"
	"sort"

	_ "modernc.org/sqlite" // 纯 Go SQLite 驱动（免 CGO）
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Open 打开 SQLite 数据库并应用推荐 PRAGMA。
func Open(path string) (*sql.DB, error) {
	var dsn string
	if path == ":memory:" {
		dsn = "file::memory:?_pragma=busy_timeout(5000)"
	} else {
		dsn = "file:" + filepath.ToSlash(path) +
			"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=synchronous(NORMAL)"
	}
	d, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	d.SetMaxOpenConns(4)
	d.SetMaxIdleConns(4)
	if err := d.Ping(); err != nil {
		_ = d.Close()
		return nil, err
	}
	return d, nil
}

// Migrate 按文件名顺序执行内嵌迁移脚本（脚本本身幂等）。
func Migrate(d *sql.DB) error {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		content, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}
		if _, err := d.Exec(string(content)); err != nil {
			return fmt.Errorf("执行迁移 %s 失败: %w", name, err)
		}
	}
	return nil
}
