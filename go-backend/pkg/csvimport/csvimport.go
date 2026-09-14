// Package csvimport 提供数据源 CSV 批量导入解析。
//
// 列头对齐 DBCheck connectInfo：db_type,name,host,port,db_name,user,password,...
// 支持中英文常见别名与可选隧道字段。
package csvimport

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Row 解析后的一行（字段保持原始字符串，校验在服务层完成）。
type Row struct {
	Line           int
	DbType         string
	Name           string
	Host           string
	Port           string
	DbName         string
	Username       string
	Password       string
	GroupName      string
	TunnelType     string
	TunnelHost     string
	TunnelPort     string
	TunnelUser     string
	TunnelPassword string
}

// RowError 行级错误（带 CSV 行号）。
type RowError struct {
	Line    int    `json:"line"`
	Message string `json:"message"`
}

var aliases = map[string]string{
	"db_type": "db_type", "type": "db_type", "dbtype": "db_type",
	"name": "name", "ds_name": "name",
	"host": "host", "ip": "host", "hostname": "host",
	"port": "port",
	"db_name": "db_name", "database": "db_name", "dbname": "db_name", "service_name": "db_name",
	"username": "username", "user": "username", "user_name": "username",
	"password": "password", "passwd": "password", "pwd": "password",
	"group_name": "group_name", "group": "group_name",
	"tunnel_type": "tunnel_type",
	"tunnel_host": "tunnel_host",
	"tunnel_port": "tunnel_port",
	"tunnel_user": "tunnel_user",
	"tunnel_password": "tunnel_password",
}

// Parse 解析 CSV 内容，返回有效行与行级错误列表；缺少必需列时返回整体错误。
func Parse(r io.Reader) ([]Row, []RowError, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	cr.TrimLeadingSpace = true

	first, err := cr.Read()
	if err != nil {
		return nil, nil, errors.New("CSV 内容为空或无法解析")
	}
	if len(first) > 0 {
		first[0] = strings.TrimPrefix(first[0], "\uFEFF") // 去 BOM
	}
	colIdx := map[string]int{}
	for i, h := range first {
		key := strings.ToLower(strings.TrimSpace(h))
		if mapped, ok := aliases[key]; ok {
			colIdx[mapped] = i
		}
	}
	for _, req := range []string{"db_type", "name", "host", "port"} {
		if _, ok := colIdx[req]; !ok {
			return nil, nil, fmt.Errorf("CSV 缺少必需列: %s", req)
		}
	}

	var rows []Row
	var errs []RowError
	line := 1
	for {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		line++
		if err != nil {
			errs = append(errs, RowError{Line: line, Message: "行解析失败: " + err.Error()})
			continue
		}
		get := func(key string) string {
			if i, ok := colIdx[key]; ok && i < len(rec) {
				return strings.TrimSpace(rec[i])
			}
			return ""
		}
		row := Row{
			Line: line,
			DbType: get("db_type"), Name: get("name"), Host: get("host"), Port: get("port"),
			DbName: get("db_name"), Username: get("username"), Password: get("password"),
			GroupName: get("group_name"),
			TunnelType: get("tunnel_type"), TunnelHost: get("tunnel_host"),
			TunnelPort: get("tunnel_port"), TunnelUser: get("tunnel_user"), TunnelPassword: get("tunnel_password"),
		}
		if row.DbType == "" || row.Name == "" || row.Host == "" {
			errs = append(errs, RowError{Line: line, Message: "db_type / name / host 不能为空"})
			continue
		}
		if row.Port != "" {
			if _, err := strconv.Atoi(row.Port); err != nil {
				errs = append(errs, RowError{Line: line, Message: "端口必须是数字"})
				continue
			}
		}
		rows = append(rows, row)
	}
	return rows, errs, nil
}
