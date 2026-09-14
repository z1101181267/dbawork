package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	pb "dbawork/proto/dbaworkv1"

	"dbawork/internal/catalog"
	"dbawork/internal/crypto"
	"dbawork/internal/grpcclient"
	"dbawork/internal/model"
	"dbawork/internal/repository"
	"dbawork/pkg/csvimport"
)

// DatasourceService 数据源业务编排。
type DatasourceService struct {
	repo      *repository.DatasourceRepo
	groupRepo *repository.GroupRepo
	dtRepo    *repository.DbTypeRepo
	cipher    *crypto.Cipher
	grpc      *grpcclient.Client
}

// NewDatasourceService 构造。
func NewDatasourceService(repo *repository.DatasourceRepo, groupRepo *repository.GroupRepo,
	dtRepo *repository.DbTypeRepo, cipher *crypto.Cipher, grpc *grpcclient.Client) *DatasourceService {
	return &DatasourceService{repo: repo, groupRepo: groupRepo, dtRepo: dtRepo, cipher: cipher, grpc: grpc}
}

// TestResultView 连接测试结果（对外 JSON 结构）。
type TestResultView struct {
	Ok        bool   `json:"ok"`
	Error     string `json:"error"`
	DbVersion string `json:"db_version"`
	LatencyMs int    `json:"latency_ms"`
	TestedAt  string `json:"tested_at"`
}

// ImportResult CSV 导入结果。
type ImportResult struct {
	SuccessCount int                  `json:"success_count"`
	FailCount    int                  `json:"fail_count"`
	Errors       []csvimport.RowError `json:"errors"`
}

// List 数据源列表。
func (s *DatasourceService) List(ctx context.Context, f repository.DSFilter) ([]model.DataSource, error) {
	return s.repo.List(ctx, f)
}

// Get 数据源详情。
func (s *DatasourceService) Get(ctx context.Context, id int64) (*model.DataSource, error) {
	ds, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if ds == nil {
		return nil, errors.New("数据源不存在")
	}
	return ds, nil
}

// validate 校验并规范化输入（就地补默认值）。
func (s *DatasourceService) validate(ctx context.Context, in *model.DataSourceInput) error {
	in.Name = strings.TrimSpace(in.Name)
	in.DbType = strings.TrimSpace(in.DbType)
	in.Host = strings.TrimSpace(in.Host)
	in.Username = strings.TrimSpace(in.Username)
	in.TunnelType = strings.TrimSpace(in.TunnelType)
	in.TunnelHost = strings.TrimSpace(in.TunnelHost)
	in.TunnelUser = strings.TrimSpace(in.TunnelUser)

	if in.Name == "" {
		return errors.New("名称不能为空")
	}
	if in.DbType == "" {
		return errors.New("数据库类型不能为空")
	}
	if !catalog.Exists(in.DbType) {
		ok, err := s.dtRepo.CustomExists(ctx, in.DbType)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("未知的数据库类型: %s", in.DbType)
		}
	}

	isSQLite := in.DbType == "sqlite"
	if !isSQLite && in.Host == "" {
		return errors.New("主机不能为空")
	}
	if isSQLite && in.Host == "" {
		in.Host = "localhost"
	}
	if in.Port == 0 {
		if e, ok := catalog.Get(in.DbType); ok {
			in.Port = e.DefaultPort
		}
	}
	if in.Port < 0 || in.Port > 65535 {
		return errors.New("端口必须在 0-65535 之间")
	}
	if !isSQLite && in.Port == 0 {
		return errors.New("端口不能为空")
	}

	switch in.TunnelType {
	case "", "none":
		in.TunnelType = "none"
	case "ssh":
		if in.TunnelHost == "" {
			return errors.New("SSH 隧道主机不能为空")
		}
		if in.TunnelUser == "" {
			return errors.New("SSH 隧道用户名不能为空")
		}
		if in.TunnelPort == 0 {
			in.TunnelPort = 22
		}
	case "winrm":
		if in.TunnelHost == "" {
			return errors.New("WinRM 隧道主机不能为空")
		}
		if in.TunnelUser == "" {
			return errors.New("WinRM 隧道用户名不能为空")
		}
		if in.TunnelTransport == "" {
			in.TunnelTransport = "http"
		}
		if in.TunnelTransport != "http" && in.TunnelTransport != "https" {
			return errors.New("WinRM 传输协议只能是 http/https")
		}
		if in.TunnelPort == 0 {
			if in.TunnelTransport == "https" {
				in.TunnelPort = 5986
			} else {
				in.TunnelPort = 5985
			}
		}
		if in.TunnelAuthScheme == "" {
			in.TunnelAuthScheme = "ntlm"
		}
		switch in.TunnelAuthScheme {
		case "basic", "ntlm", "kerberos":
		default:
			return errors.New("WinRM 认证方式只能是 basic/ntlm/kerberos")
		}
		if in.TunnelTransport == "https" {
			in.TunnelUseSSL = true
		}
	default:
		return errors.New("隧道类型只能是 none/ssh/winrm")
	}

	if strings.TrimSpace(in.ExtraParams) != "" {
		var m map[string]any
		if err := json.Unmarshal([]byte(in.ExtraParams), &m); err != nil {
			return errors.New("扩展参数必须是合法的 JSON 对象")
		}
		b, err := json.Marshal(m)
		if err != nil {
			return err
		}
		in.ExtraParams = string(b)
	}
	return nil
}

func extraToStringMap(raw string) map[string]string {
	out := map[string]string{}
	if strings.TrimSpace(raw) == "" {
		return out
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return out
	}
	for k, v := range m {
		if sv, ok := v.(string); ok {
			out[k] = sv
		} else {
			b, _ := json.Marshal(v)
			out[k] = strings.Trim(string(b), `"`)
		}
	}
	return out
}

func (s *DatasourceService) createOne(ctx context.Context, in *model.DataSourceInput) (*model.DataSource, error) {
	if err := s.validate(ctx, in); err != nil {
		return nil, err
	}
	ds := &model.DataSource{
		Name: in.Name, GroupID: in.GroupID, DbType: in.DbType, Host: in.Host, Port: in.Port,
		DbName: in.DbName, Username: in.Username, ExtraParams: in.ExtraParams,
		TunnelType: in.TunnelType, TunnelHost: in.TunnelHost, TunnelPort: in.TunnelPort,
		TunnelUser: in.TunnelUser, TunnelAuthScheme: in.TunnelAuthScheme, TunnelTransport: in.TunnelTransport,
	}
	if in.TunnelUseSSL {
		ds.TunnelUseSSL = 1
	}
	var err error
	if in.Password != "" {
		if ds.PasswordEnc, ds.PasswordIV, err = s.cipher.EncryptString(in.Password); err != nil {
			return nil, err
		}
	}
	if in.TunnelPassword != "" {
		if ds.TunnelPasswordEnc, ds.TunnelPasswordIV, err = s.cipher.EncryptString(in.TunnelPassword); err != nil {
			return nil, err
		}
	}
	if in.TunnelPrivateKey != "" {
		if ds.TunnelKeyEnc, ds.TunnelKeyIV, err = s.cipher.EncryptString(in.TunnelPrivateKey); err != nil {
			return nil, err
		}
	}
	if in.TunnelKeyPassphrase != "" {
		if ds.TunnelKeyPassEnc, ds.TunnelKeyPassIV, err = s.cipher.EncryptString(in.TunnelKeyPassphrase); err != nil {
			return nil, err
		}
	}
	if err := s.repo.Create(ctx, ds); err != nil {
		return nil, err
	}
	return ds, nil
}

// Create 新建数据源。
func (s *DatasourceService) Create(ctx context.Context, in *model.DataSourceInput) (*model.DataSource, error) {
	return s.createOne(ctx, in)
}

// Update 更新数据源（密码/密钥留空表示保持原值）。
func (s *DatasourceService) Update(ctx context.Context, id int64, in *model.DataSourceInput) (*model.DataSource, error) {
	old, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if old == nil {
		return nil, errors.New("数据源不存在")
	}
	if err := s.validate(ctx, in); err != nil {
		return nil, err
	}
	ds := &model.DataSource{
		ID: id, Name: in.Name, GroupID: in.GroupID, DbType: in.DbType, Host: in.Host, Port: in.Port,
		DbName: in.DbName, Username: in.Username, ExtraParams: in.ExtraParams,
		TunnelType: in.TunnelType, TunnelHost: in.TunnelHost, TunnelPort: in.TunnelPort,
		TunnelUser: in.TunnelUser, TunnelAuthScheme: in.TunnelAuthScheme, TunnelTransport: in.TunnelTransport,

		PasswordEnc: old.PasswordEnc, PasswordIV: old.PasswordIV,
		TunnelPasswordEnc: old.TunnelPasswordEnc, TunnelPasswordIV: old.TunnelPasswordIV,
		TunnelKeyEnc: old.TunnelKeyEnc, TunnelKeyIV: old.TunnelKeyIV,
		TunnelKeyPassEnc: old.TunnelKeyPassEnc, TunnelKeyPassIV: old.TunnelKeyPassIV,
	}
	if in.TunnelUseSSL {
		ds.TunnelUseSSL = 1
	}
	if in.Password != "" {
		if ds.PasswordEnc, ds.PasswordIV, err = s.cipher.EncryptString(in.Password); err != nil {
			return nil, err
		}
	}
	if in.TunnelPassword != "" {
		if ds.TunnelPasswordEnc, ds.TunnelPasswordIV, err = s.cipher.EncryptString(in.TunnelPassword); err != nil {
			return nil, err
		}
	}
	if in.TunnelPrivateKey != "" {
		if ds.TunnelKeyEnc, ds.TunnelKeyIV, err = s.cipher.EncryptString(in.TunnelPrivateKey); err != nil {
			return nil, err
		}
	}
	if in.TunnelKeyPassphrase != "" {
		if ds.TunnelKeyPassEnc, ds.TunnelKeyPassIV, err = s.cipher.EncryptString(in.TunnelKeyPassphrase); err != nil {
			return nil, err
		}
	}
	if err := s.repo.Update(ctx, ds); err != nil {
		return nil, err
	}
	return s.Get(ctx, id)
}

// Delete 删除数据源。
func (s *DatasourceService) Delete(ctx context.Context, id int64) error {
	old, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if old == nil {
		return errors.New("数据源不存在")
	}
	return s.repo.Delete(ctx, id)
}

func (s *DatasourceService) decryptField(ct, iv []byte) (string, error) {
	v, err := s.cipher.DecryptString(ct, iv)
	if err != nil {
		return "", fmt.Errorf("凭据解密失败: %w", err)
	}
	return v, nil
}

func (s *DatasourceService) toProto(ds *model.DataSource) (*pb.DataSourceConn, error) {
	conn := &pb.DataSourceConn{
		Id: int32(ds.ID), DbType: ds.DbType, Host: ds.Host, Port: int32(ds.Port),
		DbName: ds.DbName, Username: ds.Username, Extra: extraToStringMap(ds.ExtraParams),
	}
	var err error
	if conn.Password, err = s.decryptField(ds.PasswordEnc, ds.PasswordIV); err != nil {
		return nil, err
	}
	if ds.TunnelType != "" && ds.TunnelType != "none" {
		t := &pb.ServerTunnel{
			TunnelType: ds.TunnelType, Host: ds.TunnelHost, Port: int32(ds.TunnelPort),
			User: ds.TunnelUser, AuthScheme: ds.TunnelAuthScheme, Transport: ds.TunnelTransport,
			UseSsl: ds.TunnelUseSSL == 1,
		}
		if t.Password, err = s.decryptField(ds.TunnelPasswordEnc, ds.TunnelPasswordIV); err != nil {
			return nil, err
		}
		if t.PrivateKey, err = s.decryptField(ds.TunnelKeyEnc, ds.TunnelKeyIV); err != nil {
			return nil, err
		}
		if t.KeyPassphrase, err = s.decryptField(ds.TunnelKeyPassEnc, ds.TunnelKeyPassIV); err != nil {
			return nil, err
		}
		conn.Tunnel = t
	}
	return conn, nil
}

func (s *DatasourceService) inputToProto(in *model.DataSourceInput) *pb.DataSourceConn {
	conn := &pb.DataSourceConn{
		DbType: in.DbType, Host: in.Host, Port: int32(in.Port),
		DbName: in.DbName, Username: in.Username, Password: in.Password,
		Extra: extraToStringMap(in.ExtraParams),
	}
	if in.TunnelType != "" && in.TunnelType != "none" {
		conn.Tunnel = &pb.ServerTunnel{
			TunnelType: in.TunnelType, Host: in.TunnelHost, Port: int32(in.TunnelPort),
			User: in.TunnelUser, Password: in.TunnelPassword,
			PrivateKey: in.TunnelPrivateKey, KeyPassphrase: in.TunnelKeyPassphrase,
			AuthScheme: in.TunnelAuthScheme, Transport: in.TunnelTransport, UseSsl: in.TunnelUseSSL,
		}
	}
	return conn
}

// TestByID 测试已保存数据源（解密 → gRPC → 回写状态）。
func (s *DatasourceService) TestByID(ctx context.Context, id int64) (*TestResultView, error) {
	ds, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if ds == nil {
		return nil, errors.New("数据源不存在")
	}

	view := &TestResultView{TestedAt: nowRFC3339()}
	conn, err := s.toProto(ds)
	if err != nil {
		view.Error = err.Error()
		_ = s.repo.UpdateTestResult(ctx, id, 2, view.TestedAt, truncate(view.Error, 2000))
		return view, nil
	}
	cli, err := s.grpc.Connection()
	if err != nil {
		view.Error = "连接 Python 运行时失败: " + err.Error()
		_ = s.repo.UpdateTestResult(ctx, id, 2, view.TestedAt, truncate(view.Error, 2000))
		return view, nil
	}
	res, err := cli.TestConnection(ctx, conn)
	if err != nil {
		view.Error = "调用连接测试失败: " + err.Error()
		_ = s.repo.UpdateTestResult(ctx, id, 2, view.TestedAt, truncate(view.Error, 2000))
		return view, nil
	}
	view.Ok = res.Ok
	view.Error = res.Error
	view.DbVersion = res.DbVersion
	view.LatencyMs = int(res.LatencyMs)
	if res.TestedAt != "" {
		view.TestedAt = res.TestedAt
	}
	status := 1
	if !res.Ok {
		status = 2
	}
	_ = s.repo.UpdateTestResult(ctx, id, status, view.TestedAt, truncate(view.Error, 2000))
	return view, nil
}

// TestAdhoc 未保存前的临时连接测试。
func (s *DatasourceService) TestAdhoc(ctx context.Context, in *model.DataSourceInput) (*TestResultView, error) {
	if err := s.validate(ctx, in); err != nil {
		return nil, err
	}
	view := &TestResultView{TestedAt: nowRFC3339()}
	cli, err := s.grpc.Connection()
	if err != nil {
		view.Error = "连接 Python 运行时失败: " + err.Error()
		return view, nil
	}
	res, err := cli.TestConnection(ctx, s.inputToProto(in))
	if err != nil {
		view.Error = "调用连接测试失败: " + err.Error()
		return view, nil
	}
	view.Ok = res.Ok
	view.Error = res.Error
	view.DbVersion = res.DbVersion
	view.LatencyMs = int(res.LatencyMs)
	if res.TestedAt != "" {
		view.TestedAt = res.TestedAt
	}
	return view, nil
}

func (s *DatasourceService) ensureGroup(ctx context.Context, name string) (int64, error) {
	g, err := s.groupRepo.FindByName(ctx, name)
	if err != nil {
		return 0, err
	}
	if g != nil {
		return g.ID, nil
	}
	ng := &model.Group{Name: name}
	if err := s.groupRepo.Create(ctx, ng); err != nil {
		return 0, err
	}
	return ng.ID, nil
}

// Import CSV 批量导入。
func (s *DatasourceService) Import(ctx context.Context, data []byte) (*ImportResult, error) {
	rows, rowErrs, err := csvimport.Parse(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	res := &ImportResult{Errors: []csvimport.RowError{}}
	for _, re := range rowErrs {
		res.FailCount++
		res.Errors = append(res.Errors, re)
	}
	for _, row := range rows {
		in := &model.DataSourceInput{
			Name: row.Name, DbType: row.DbType, Host: row.Host, DbName: row.DbName,
			Username: row.Username, Password: row.Password,
			TunnelType: row.TunnelType, TunnelHost: row.TunnelHost,
			TunnelUser: row.TunnelUser, TunnelPassword: row.TunnelPassword,
		}
		if v, err := strconv.Atoi(strings.TrimSpace(row.Port)); err == nil {
			in.Port = v
		}
		if v, err := strconv.Atoi(strings.TrimSpace(row.TunnelPort)); err == nil {
			in.TunnelPort = v
		}
		if g := strings.TrimSpace(row.GroupName); g != "" {
			gid, err := s.ensureGroup(ctx, g)
			if err != nil {
				res.FailCount++
				res.Errors = append(res.Errors, csvimport.RowError{Line: row.Line, Message: "分组处理失败: " + err.Error()})
				continue
			}
			in.GroupID = gid
		}
		if _, err := s.createOne(ctx, in); err != nil {
			res.FailCount++
			res.Errors = append(res.Errors, csvimport.RowError{Line: row.Line, Message: err.Error()})
			continue
		}
		res.SuccessCount++
	}
	return res, nil
}

// ExportCSV 导出（不含密码）。
func (s *DatasourceService) ExportCSV(ctx context.Context) ([]byte, error) {
	rows, err := s.repo.ListForExport(ctx)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	buf.WriteString("\xEF\xBB\xBF") // UTF-8 BOM，便于 Excel 打开
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"db_type", "name", "host", "port", "db_name", "username",
		"group_name", "tunnel_type", "tunnel_host", "tunnel_port", "tunnel_user"})
	for _, e := range rows {
		tp := ""
		if e.TunnelPort > 0 {
			tp = strconv.Itoa(e.TunnelPort)
		}
		_ = w.Write([]string{e.DbType, e.Name, e.Host, strconv.Itoa(e.Port), e.DbName,
			e.Username, e.GroupName, e.TunnelType, e.TunnelHost, tp, e.TunnelUser})
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func nowRFC3339() string { return time.Now().UTC().Format(time.RFC3339) }

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}
