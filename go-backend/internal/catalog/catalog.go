// Package catalog 加载内置数据库类型目录（对标 DBCheck DB_TYPE_CATALOG）。
//
// 与 py-backend/config/db_type_catalog.json 保持同一份内容，修改时需同步。
package catalog

import (
	_ "embed"
	"encoding/json"
	"sort"
)

//go:embed db_type_catalog.json
var raw []byte

// Entry 目录条目。
type Entry struct {
	Key             string `json:"key"`
	NameZh          string `json:"name_zh"`
	NameEn          string `json:"name_en"`
	DriverClassHint string `json:"driver_class_hint"`
	IsJdbc          bool   `json:"is_jdbc"`
	Order           int    `json:"order"`
	DriverKind      string `json:"driver_kind"`
	PackageName     string `json:"package_name"`
	DefaultPort     int    `json:"default_port"`
}

type catalogFile struct {
	Version int     `json:"version"`
	Types   []Entry `json:"types"`
}

var builtin []Entry
var byKey = map[string]Entry{}

func init() {
	var f catalogFile
	if err := json.Unmarshal(raw, &f); err != nil {
		panic("db_type_catalog.json 解析失败: " + err.Error())
	}
	sort.Slice(f.Types, func(i, j int) bool { return f.Types[i].Order < f.Types[j].Order })
	builtin = f.Types
	for _, e := range builtin {
		byKey[e.Key] = e
	}
}

// Builtin 返回按 order 排序的内置目录。
func Builtin() []Entry { return builtin }

// Get 按 key 查找内置条目。
func Get(key string) (Entry, bool) {
	e, ok := byKey[key]
	return e, ok
}

// Exists 判断 key 是否为内置类型。
func Exists(key string) bool {
	_, ok := byKey[key]
	return ok
}
