package csvimport

import (
	"strings"
	"testing"
)

func TestParseBasicWithAliases(t *testing.T) {
	data := "\uFEFFdb_type,ds_name,ip,port,database,user,pwd,group\nmysql,库1,1.2.3.4,3306,db1,root,p1,组A\n"
	rows, errs, err := Parse(strings.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if len(errs) != 0 {
		t.Fatalf("unexpected errs: %v", errs)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(rows))
	}
	r := rows[0]
	if r.DbType != "mysql" || r.Name != "库1" || r.Host != "1.2.3.4" ||
		r.DbName != "db1" || r.Username != "root" || r.Password != "p1" || r.GroupName != "组A" {
		t.Fatalf("字段映射错误: %+v", r)
	}
}

func TestParseRowErrors(t *testing.T) {
	data := "db_type,name,host,port\nmysql,ok,1.1.1.1,3306\nmysql,,1.1.1.1,3306\nmysql,badport,1.1.1.1,abc\n"
	rows, errs, err := Parse(strings.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("valid rows want 1, got %d", len(rows))
	}
	if len(errs) != 2 {
		t.Fatalf("errs want 2, got %v", errs)
	}
	if errs[0].Line != 3 || errs[1].Line != 4 {
		t.Fatalf("行号错误: %v", errs)
	}
}

func TestParseMissingColumns(t *testing.T) {
	_, _, err := Parse(strings.NewReader("a,b,c\n1,2,3\n"))
	if err == nil {
		t.Fatal("缺列应报错")
	}
}

func TestParseTunnel(t *testing.T) {
	data := "db_type,name,host,port,tunnel_type,tunnel_host,tunnel_port,tunnel_user,tunnel_password\n" +
		"mssql,win,10.0.0.9,1433,winrm,10.0.0.10,5985,administrator,pw\n"
	rows, errs, err := Parse(strings.NewReader(data))
	if err != nil || len(errs) > 0 || len(rows) != 1 {
		t.Fatalf("err=%v errs=%v rows=%v", err, errs, rows)
	}
	if rows[0].TunnelType != "winrm" || rows[0].TunnelPort != "5985" || rows[0].TunnelUser != "administrator" {
		t.Fatal("隧道字段解析失败")
	}
}

func TestParseEmpty(t *testing.T) {
	_, _, err := Parse(strings.NewReader(""))
	if err == nil {
		t.Fatal("空内容应报错")
	}
}
