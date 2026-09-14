package repository

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"dbawork/internal/db"
	"dbawork/internal/model"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(d); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d
}

func TestGroupCRUD(t *testing.T) {
	d := newTestDB(t)
	r := NewGroupRepo(d)
	ctx := context.Background()

	g1 := &model.Group{Name: "生产库"}
	if err := r.Create(ctx, g1); err != nil {
		t.Fatal(err)
	}
	if g1.ID == 0 {
		t.Fatal("ID 应回填")
	}
	g2 := &model.Group{Name: "核心业务", ParentID: g1.ID}
	if err := r.Create(ctx, g2); err != nil {
		t.Fatal(err)
	}

	list, err := r.List(ctx)
	if err != nil || len(list) != 2 {
		t.Fatalf("list=%v err=%v", list, err)
	}

	g1.Name = "生产环境"
	if err := r.Update(ctx, g1); err != nil {
		t.Fatal(err)
	}
	got, _ := r.Get(ctx, g1.ID)
	if got == nil || got.Name != "生产环境" {
		t.Fatal("更新未生效")
	}

	n, err := r.CountChildren(ctx, g1.ID)
	if err != nil || n != 1 {
		t.Fatalf("children=%d err=%v", n, err)
	}

	desc, err := r.DescendantIDs(ctx, g1.ID)
	if err != nil || len(desc) != 2 {
		t.Fatalf("desc=%v err=%v", desc, err)
	}

	found, err := r.FindByName(ctx, "核心业务")
	if err != nil || found == nil || found.ID != g2.ID {
		t.Fatal("FindByName 失败")
	}

	if err := r.Delete(ctx, g2.ID); err != nil {
		t.Fatal(err)
	}
	if err := r.Delete(ctx, g1.ID); err != nil {
		t.Fatal(err)
	}
}

func TestDatasourceCRUDAndFilter(t *testing.T) {
	d := newTestDB(t)
	r := NewDatasourceRepo(d)
	ctx := context.Background()

	ds := &model.DataSource{
		Name: "核心库", DbType: "mysql", Host: "10.0.0.1", Port: 3306,
		DbName: "core", Username: "root", TunnelType: "none",
		PasswordEnc: []byte{1, 2, 3}, PasswordIV: []byte{4, 5, 6},
	}
	if err := r.Create(ctx, ds); err != nil {
		t.Fatal(err)
	}
	if ds.ID == 0 || ds.CreatedAt == "" {
		t.Fatal("ID/时间戳应回填")
	}

	ds2 := &model.DataSource{
		Name: "报表库", DbType: "postgresql", Host: "10.0.0.2", Port: 5432,
		Username: "readonly", TunnelType: "ssh", TunnelHost: "jump", TunnelPort: 22, TunnelUser: "ops",
	}
	if err := r.Create(ctx, ds2); err != nil {
		t.Fatal(err)
	}

	all, _ := r.List(ctx, DSFilter{})
	if len(all) != 2 {
		t.Fatalf("want 2, got %d", len(all))
	}

	byType, _ := r.List(ctx, DSFilter{DbType: "mysql"})
	if len(byType) != 1 || byType[0].Name != "核心库" {
		t.Fatal("db_type 过滤失败")
	}

	byKw, _ := r.List(ctx, DSFilter{Keyword: "报表"})
	if len(byKw) != 1 {
		t.Fatal("关键字过滤失败")
	}

	if err := r.UpdateTestResult(ctx, ds.ID, 1, "2026-01-01T00:00:00Z", ""); err != nil {
		t.Fatal(err)
	}
	got, _ := r.Get(ctx, ds.ID)
	if got.Status != 1 || got.LastTestAt == nil {
		t.Fatal("回写测试结果失败")
	}

	exp, _ := r.ListForExport(ctx)
	if len(exp) != 2 {
		t.Fatal("导出列表失败")
	}

	ds.Name = "核心库V2"
	if err := r.Update(ctx, ds); err != nil {
		t.Fatal(err)
	}
	got, _ = r.Get(ctx, ds.ID)
	if got.Name != "核心库V2" || got.UpdatedAt == "" {
		t.Fatal("更新失败")
	}

	if err := r.Delete(ctx, ds.ID); err != nil {
		t.Fatal(err)
	}
	if g, _ := r.Get(ctx, ds.ID); g != nil {
		t.Fatal("删除失败")
	}
}

func TestDriverRepo(t *testing.T) {
	d := newTestDB(t)
	r := NewDriverRepo(d)
	ctx := context.Background()

	if err := r.EnsurePythonDriver(ctx, "mysql", "pymysql"); err != nil {
		t.Fatal(err)
	}
	if err := r.EnsurePythonDriver(ctx, "mysql", "pymysql"); err != nil {
		t.Fatal(err)
	}

	list, _ := r.List(ctx, "mysql")
	if len(list) != 1 {
		t.Fatalf("want 1, got %d", len(list))
	}
	if list[0].DriverKind != "python" || list[0].PackageName != "pymysql" {
		t.Fatal("字段错误")
	}

	if err := r.UpdateRuntimeStatus(ctx, "mysql", "python", 1, "1.1.1", "ok"); err != nil {
		t.Fatal(err)
	}
	row, _ := r.GetByID(ctx, list[0].ID)
	if row.Installed != 1 || row.InstalledVersion != "1.1.1" {
		t.Fatal("状态回写失败")
	}

	jd := &model.Driver{
		DbType: "mysql", DriverKind: "jdbc", Version: "8.0.33",
		JarFilename: "mysql-connector.jar", JarPath: "/x/y.jar", FileSize: 100, Installed: 1,
	}
	if err := r.Create(ctx, jd); err != nil {
		t.Fatal(err)
	}

	if err := r.SetActive(ctx, jd.ID); err != nil {
		t.Fatal(err)
	}
	has, _ := r.HasActive(ctx, "mysql")
	if !has {
		t.Fatal("应有默认驱动")
	}
	row, _ = r.GetByID(ctx, list[0].ID)
	if row.IsActive != 0 {
		t.Fatal("默认应互斥")
	}

	if err := r.Delete(ctx, jd.ID); err != nil {
		t.Fatal(err)
	}
}

func TestDbTypeRepo(t *testing.T) {
	d := newTestDB(t)
	r := NewDbTypeRepo(d)
	ctx := context.Background()

	if err := r.Hide(ctx, "db2"); err != nil {
		t.Fatal(err)
	}
	if err := r.Hide(ctx, "db2"); err != nil {
		t.Fatal(err)
	}
	hidden, _ := r.HiddenList(ctx)
	if len(hidden) != 1 || hidden[0] != "db2" {
		t.Fatalf("hidden=%v", hidden)
	}
	is, _ := r.IsHidden(ctx, "db2")
	if !is {
		t.Fatal("IsHidden 失败")
	}
	if err := r.Restore(ctx, "db2"); err != nil {
		t.Fatal(err)
	}
	hidden, _ = r.HiddenList(ctx)
	if len(hidden) != 0 {
		t.Fatal("恢复失败")
	}

	c := &model.DbTypeCustom{DbType: "kylin", NameZh: "麒麟", NameEn: "Kylin", IsJdbc: 1}
	if err := r.CreateCustom(ctx, c); err != nil {
		t.Fatal(err)
	}
	ok, _ := r.CustomExists(ctx, "kylin")
	if !ok {
		t.Fatal("CustomExists 失败")
	}
	list, _ := r.CustomList(ctx)
	if len(list) != 1 || list[0].NameZh != "麒麟" {
		t.Fatal("CustomList 失败")
	}
	if err := r.DeleteCustom(ctx, "kylin"); err != nil {
		t.Fatal(err)
	}
	ok, _ = r.CustomExists(ctx, "kylin")
	if ok {
		t.Fatal("DeleteCustom 失败")
	}
}
