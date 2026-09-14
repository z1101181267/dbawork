package service

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"dbawork/internal/crypto"
	"dbawork/internal/db"
	"dbawork/internal/grpcclient"
	"dbawork/internal/model"
	"dbawork/internal/repository"
)

func newServices(t *testing.T) (*DatasourceService, *GroupService, *repository.DriverRepo) {
	t.Helper()
	d, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(d); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })

	groupRepo := repository.NewGroupRepo(d)
	dsRepo := repository.NewDatasourceRepo(d)
	driverRepo := repository.NewDriverRepo(d)
	dtRepo := repository.NewDbTypeRepo(d)

	cipher, err := crypto.New(make([]byte, crypto.KeySize))
	if err != nil {
		t.Fatal(err)
	}

	cli := grpcclient.New("127.0.0.1:1")
	dsSvc := NewDatasourceService(dsRepo, groupRepo, dtRepo, cipher, cli)
	groupSvc := NewGroupService(groupRepo, dsRepo)
	driverSvc := NewDriverService(driverRepo, dtRepo, cli)
	if err := driverSvc.SeedBuiltin(context.Background()); err != nil {
		t.Fatal(err)
	}
	return dsSvc, groupSvc, driverRepo
}

func TestSeedBuiltinDrivers(t *testing.T) {
	_, _, driverRepo := newServices(t)
	list, err := driverRepo.List(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) == 0 {
		t.Fatal("内置驱动应已初始化")
	}
	found := false
	for _, d := range list {
		if d.DbType == "mysql" && d.PackageName == "pymysql" {
			found = true
		}
	}
	if !found {
		t.Fatal("缺少 mysql/pymysql 驱动行")
	}
}

func TestDatasourceCreateAndValidation(t *testing.T) {
	dsSvc, _, _ := newServices(t)
	ctx := context.Background()

	if _, err := dsSvc.Create(ctx, &model.DataSourceInput{DbType: "mysql", Host: "h", Port: 3306}); err == nil {
		t.Fatal("应校验名称")
	}
	if _, err := dsSvc.Create(ctx, &model.DataSourceInput{Name: "x", DbType: "unknown_db", Host: "h", Port: 1}); err == nil {
		t.Fatal("应校验类型")
	}
	if _, err := dsSvc.Create(ctx, &model.DataSourceInput{Name: "x", DbType: "mysql", Host: "h", Port: 3306, ExtraParams: "{bad"}); err == nil {
		t.Fatal("应校验扩展参数 JSON")
	}
	if _, err := dsSvc.Create(ctx, &model.DataSourceInput{Name: "t", DbType: "mysql", Host: "h", TunnelType: "ssh"}); err == nil {
		t.Fatal("ssh 隧道缺主机应报错")
	}
	if _, err := dsSvc.Create(ctx, &model.DataSourceInput{Name: "t2", DbType: "mssql", Host: "h", Port: 1433, TunnelType: "winrm"}); err == nil {
		t.Fatal("winrm 隧道缺主机应报错")
	}

	ds, err := dsSvc.Create(ctx, &model.DataSourceInput{
		Name: "测试库", DbType: "mysql", Host: "127.0.0.1", Port: 3306,
		Username: "root", Password: "secret", ExtraParams: `{"charset":"utf8mb4"}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if ds.PasswordEnc == nil {
		t.Fatal("密码应加密落库")
	}

	// 端口默认值（未填端口时使用目录默认端口）
	ds2, err := dsSvc.Create(ctx, &model.DataSourceInput{Name: "默认端口", DbType: "mysql", Host: "127.0.0.1", Username: "u"})
	if err != nil {
		t.Fatal(err)
	}
	if ds2.Port != 3306 {
		t.Fatalf("默认端口应为 3306, got %d", ds2.Port)
	}

	// 更新：密码留空保持原值
	upd, err := dsSvc.Update(ctx, ds.ID, &model.DataSourceInput{
		Name: "测试库2", DbType: "mysql", Host: "127.0.0.1", Port: 3307, Username: "root",
	})
	if err != nil {
		t.Fatal(err)
	}
	if upd.Name != "测试库2" || upd.Port != 3307 {
		t.Fatal("更新未生效")
	}
	if string(upd.PasswordEnc) != string(ds.PasswordEnc) {
		t.Fatal("密码留空应保持")
	}

	// 导出（不含密码）
	csvBytes, err := dsSvc.ExportCSV(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(csvBytes), "测试库2") {
		t.Fatal("导出缺少记录")
	}
	if strings.Contains(string(csvBytes), "secret") {
		t.Fatal("导出不应包含密码")
	}
}

func TestGroupDeleteGuard(t *testing.T) {
	dsSvc, groupSvc, _ := newServices(t)
	ctx := context.Background()

	g, err := groupSvc.Create(ctx, &model.Group{Name: "沙箱"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = dsSvc.Create(ctx, &model.DataSourceInput{
		Name: "db", DbType: "sqlite", Host: "localhost", DbName: ":memory:", GroupID: g.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := groupSvc.Delete(ctx, g.ID); err == nil {
		t.Fatal("分组非空应拒绝删除")
	}

	tree, err := groupSvc.Tree(ctx)
	if err != nil || len(tree) != 1 {
		t.Fatalf("树构建失败: %v", err)
	}
}

func TestImport(t *testing.T) {
	dsSvc, _, _ := newServices(t)
	ctx := context.Background()

	csvData := "db_type,name,host,port,db_name,user,password,group_name\n" +
		"mysql,订单库,10.0.0.3,3306,orders,root,pw1,生产\n" +
		"postgresql,分析库,10.0.0.4,5432,analytics,reader,pw2,\n" +
		"unknown_type,坏类型,10.0.0.5,3306,,u,p,\n"
	res, err := dsSvc.Import(ctx, []byte(csvData))
	if err != nil {
		t.Fatal(err)
	}
	if res.SuccessCount != 2 {
		t.Fatalf("成功数=%d（期望2）", res.SuccessCount)
	}
	if res.FailCount != 1 {
		t.Fatalf("失败数=%d（期望1）", res.FailCount)
	}
	if len(res.Errors) != 1 || res.Errors[0].Line != 4 {
		t.Fatalf("错误行号应为 4: %+v", res.Errors)
	}
}
