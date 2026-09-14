// Package service 业务编排层。
package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	pb "dbawork/proto/dbaworkv1"

	"dbawork/internal/catalog"
	"dbawork/internal/grpcclient"
	"dbawork/internal/model"
	"dbawork/internal/repository"
)

// DriverService 驱动管理业务。
type DriverService struct {
	repo   *repository.DriverRepo
	dtRepo *repository.DbTypeRepo
	grpc   *grpcclient.Client
}

// NewDriverService 构造。
func NewDriverService(repo *repository.DriverRepo, dtRepo *repository.DbTypeRepo, grpc *grpcclient.Client) *DriverService {
	return &DriverService{repo: repo, dtRepo: dtRepo, grpc: grpc}
}

// SeedBuiltin 启动时幂等写入内置 python 驱动行。
func (s *DriverService) SeedBuiltin(ctx context.Context) error {
	for _, e := range catalog.Builtin() {
		if e.DriverKind == "python" && e.PackageName != "" {
			if err := s.repo.EnsurePythonDriver(ctx, e.Key, e.PackageName); err != nil {
				return err
			}
		}
	}
	return nil
}

// ListDrivers 驱动列表。
func (s *DriverService) ListDrivers(ctx context.Context, dbType string) ([]model.Driver, error) {
	return s.repo.List(ctx, dbType)
}

// DetectAll 全量检测并回写。
func (s *DriverService) DetectAll(ctx context.Context) (map[string]int, error) {
	cli, err := s.grpc.Driver()
	if err != nil {
		return nil, fmt.Errorf("连接 Python 运行时失败: %w", err)
	}
	resp, err := cli.DetectAll(ctx, &pb.Empty{})
	if err != nil {
		return nil, fmt.Errorf("调用 DetectAll 失败: %w", err)
	}
	for _, st := range resp.Statuses {
		inst := 0
		if st.Installed {
			inst = 1
		}
		if err := s.repo.UpdateRuntimeStatus(ctx, st.DbType, st.DriverKind, inst, st.InstalledVersion, st.Note); err != nil {
			return nil, err
		}
	}
	return s.countStatus(ctx)
}

func (s *DriverService) countStatus(ctx context.Context) (map[string]int, error) {
	list, err := s.repo.List(ctx, "")
	if err != nil {
		return nil, err
	}
	total, installed := 0, 0
	for _, d := range list {
		total++
		if d.Installed == 1 {
			installed++
		}
	}
	return map[string]int{"total": total, "installed": installed}, nil
}

// DetectOne 单类型检测并回写，返回该类型下的驱动行。
func (s *DriverService) DetectOne(ctx context.Context, dbType string) ([]model.Driver, error) {
	if e, ok := catalog.Get(dbType); ok && e.DriverKind == "python" && e.PackageName != "" {
		if err := s.repo.EnsurePythonDriver(ctx, dbType, e.PackageName); err != nil {
			return nil, err
		}
	}
	cli, err := s.grpc.Driver()
	if err != nil {
		return nil, fmt.Errorf("连接 Python 运行时失败: %w", err)
	}
	st, err := cli.DetectOne(ctx, &pb.DetectOneRequest{DbType: dbType})
	if err != nil {
		return nil, fmt.Errorf("调用 DetectOne 失败: %w", err)
	}
	inst := 0
	if st.Installed {
		inst = 1
	}
	if err := s.repo.UpdateRuntimeStatus(ctx, st.DbType, st.DriverKind, inst, st.InstalledVersion, st.Note); err != nil {
		return nil, err
	}
	return s.repo.List(ctx, dbType)
}

// Install 安装 python 驱动（完成后自动重新检测）。
func (s *DriverService) Install(ctx context.Context, dbType, kind, pkg, version string) (bool, string, error) {
	if dbType == "" || pkg == "" {
		return false, "", errors.New("db_type 与 package_name 不能为空")
	}
	if kind == "" {
		kind = "python"
	}
	cli, err := s.grpc.Driver()
	if err != nil {
		return false, "", fmt.Errorf("连接 Python 运行时失败: %w", err)
	}
	res, err := cli.InstallDriver(ctx, &pb.InstallDriverRequest{
		DbType: dbType, DriverKind: kind, PackageName: pkg, Version: version,
	})
	if err != nil {
		return false, "", fmt.Errorf("调用 InstallDriver 失败: %w", err)
	}
	if res.Ok {
		_, _ = s.DetectOne(context.WithoutCancel(ctx), dbType)
	}
	return res.Ok, res.Message, nil
}

// Uninstall 卸载 python 驱动。
func (s *DriverService) Uninstall(ctx context.Context, dbType, kind string) (bool, string, error) {
	cli, err := s.grpc.Driver()
	if err != nil {
		return false, "", fmt.Errorf("连接 Python 运行时失败: %w", err)
	}
	res, err := cli.UninstallDriver(ctx, &pb.UninstallDriverRequest{DbType: dbType, DriverKind: kind})
	if err != nil {
		return false, "", fmt.Errorf("调用 UninstallDriver 失败: %w", err)
	}
	if res.Ok {
		_, _ = s.DetectOne(context.WithoutCancel(ctx), dbType)
	}
	return res.Ok, res.Message, nil
}

// UploadJar 上传 JAR 并落库（首个 JAR 自动设为默认）。
func (s *DriverService) UploadJar(ctx context.Context, dbType, version, driverClass, filename string, data []byte) (*model.Driver, error) {
	if dbType == "" || filename == "" || len(data) == 0 {
		return nil, errors.New("db_type、文件名与文件内容不能为空")
	}
	cli, err := s.grpc.Driver()
	if err != nil {
		return nil, fmt.Errorf("连接 Python 运行时失败: %w", err)
	}
	reply, err := cli.UploadJar(ctx, &pb.UploadJarRequest{
		DbType: dbType, Version: version, DriverClass: driverClass,
		JarFilename: filename, JarBytes: data,
	})
	if err != nil {
		return nil, fmt.Errorf("调用 UploadJar 失败: %w", err)
	}
	if !reply.Ok {
		return nil, errors.New(reply.Message)
	}
	d := &model.Driver{
		DbType: dbType, DriverKind: "jdbc", Version: version,
		DriverClass: driverClass, JarFilename: filename,
		JarPath: reply.JarPath, FileSize: reply.FileSize,
		Installed: 1, Note: "JAR 已上传",
	}
	if err := s.repo.Create(ctx, d); err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return nil, errors.New("同类型、同版本的驱动已存在")
		}
		return nil, err
	}
	has, err := s.repo.HasActive(ctx, dbType)
	if err == nil && !has {
		_ = s.repo.SetActive(ctx, d.ID)
	}
	return s.repo.GetByID(ctx, d.ID)
}

// Delete 删除驱动（jdbc 行连带删除 JAR 文件）。
func (s *DriverService) Delete(ctx context.Context, id int64) error {
	d, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if d == nil {
		return errors.New("驱动不存在")
	}
	if d.JarPath != "" {
		cli, err := s.grpc.Driver()
		if err != nil {
			return fmt.Errorf("连接 Python 运行时失败: %w", err)
		}
		res, err := cli.DeleteJar(ctx, &pb.DeleteJarRequest{JarPath: d.JarPath})
		if err != nil {
			return fmt.Errorf("调用 DeleteJar 失败: %w", err)
		}
		if !res.Ok {
			return fmt.Errorf("删除 JAR 文件失败: %s", res.Message)
		}
	}
	return s.repo.Delete(ctx, id)
}

// Activate 设置默认驱动（DB 为准，运行时同步尽力而为）。
func (s *DriverService) Activate(ctx context.Context, id int64) (*model.Driver, string, error) {
	d, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, "", err
	}
	if d == nil {
		return nil, "", errors.New("驱动不存在")
	}
	if err := s.repo.SetActive(ctx, id); err != nil {
		return nil, "", err
	}
	syncMsg := ""
	if cli, err := s.grpc.Driver(); err == nil {
		syncCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if _, err := cli.ActivateDriver(syncCtx, &pb.ActivateDriverRequest{
			DbType: d.DbType, DriverId: int32(d.ID),
		}); err != nil {
			syncMsg = "运行时同步失败: " + err.Error()
		}
	}
	d, _ = s.repo.GetByID(ctx, id)
	return d, syncMsg, nil
}

// ListDbTypes 合并视图：内置 + 自定义 - 隐藏。
func (s *DriverService) ListDbTypes(ctx context.Context, includeHidden bool) ([]model.DbTypeView, error) {
	hiddenList, err := s.dtRepo.HiddenList(ctx)
	if err != nil {
		return nil, err
	}
	hidden := map[string]bool{}
	for _, k := range hiddenList {
		hidden[k] = true
	}
	var out []model.DbTypeView
	for _, e := range catalog.Builtin() {
		if !includeHidden && hidden[e.Key] {
			continue
		}
		out = append(out, model.DbTypeView{
			Key: e.Key, NameZh: e.NameZh, NameEn: e.NameEn,
			DriverClassHint: e.DriverClassHint, IsJdbc: e.IsJdbc,
			Order: e.Order, IsCustom: false, DefaultPort: e.DefaultPort,
			Hidden: hidden[e.Key],
		})
	}
	customs, err := s.dtRepo.CustomList(ctx)
	if err != nil {
		return nil, err
	}
	for i, c := range customs {
		out = append(out, model.DbTypeView{
			Key: c.DbType, NameZh: c.NameZh, NameEn: c.NameEn,
			DriverClassHint: c.DriverClassHint, IsJdbc: c.IsJdbc == 1,
			Order: 1000 + i, IsCustom: true, DefaultPort: 0, Hidden: false,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Order != out[j].Order {
			return out[i].Order < out[j].Order
		}
		return out[i].Key < out[j].Key
	})
	return out, nil
}

var dbTypeKeyRe = regexp.MustCompile(`^[a-z][a-z0-9_]{1,31}$`)

// CreateCustomType 新建自定义数据库类型。
func (s *DriverService) CreateCustomType(ctx context.Context, c *model.DbTypeCustom) error {
	if !dbTypeKeyRe.MatchString(c.DbType) {
		return errors.New("类型 key 只能由小写字母、数字、下划线组成（2-32 位）")
	}
	if c.NameZh == "" || c.NameEn == "" {
		return errors.New("中文名与英文名不能为空")
	}
	if catalog.Exists(c.DbType) {
		return errors.New("该 key 与内置类型重复")
	}
	exists, err := s.dtRepo.CustomExists(ctx, c.DbType)
	if err != nil {
		return err
	}
	if exists {
		return errors.New("该自定义类型已存在")
	}
	return s.dtRepo.CreateCustom(ctx, c)
}

// HideOrDeleteType 隐藏内置类型 / 删除自定义类型。
func (s *DriverService) HideOrDeleteType(ctx context.Context, key string) error {
	if catalog.Exists(key) {
		return s.dtRepo.Hide(ctx, key)
	}
	exists, err := s.dtRepo.CustomExists(ctx, key)
	if err != nil {
		return err
	}
	if !exists {
		return errors.New("类型不存在")
	}
	return s.dtRepo.DeleteCustom(ctx, key)
}

// RestoreType 恢复被隐藏的内置类型。
func (s *DriverService) RestoreType(ctx context.Context, key string) error {
	return s.dtRepo.Restore(ctx, key)
}
