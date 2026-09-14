package repository

import (
	"context"
	"database/sql"
	"errors"

	"dbawork/internal/model"
)

const dsCols = `id, name, group_id, db_type, host, port, db_name, username,
	password_enc, password_iv, extra_params,
	tunnel_type, tunnel_host, tunnel_port, tunnel_user,
	tunnel_password_enc, tunnel_password_iv, tunnel_key_enc, tunnel_key_iv,
	tunnel_key_pass_enc, tunnel_key_pass_iv, tunnel_auth_scheme, tunnel_transport, tunnel_use_ssl,
	status, last_test_at, last_error, created_at, updated_at`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanDataSource(row rowScanner) (*model.DataSource, error) {
	var ds model.DataSource
	var lastTest sql.NullString
	err := row.Scan(&ds.ID, &ds.Name, &ds.GroupID, &ds.DbType, &ds.Host, &ds.Port, &ds.DbName, &ds.Username,
		&ds.PasswordEnc, &ds.PasswordIV, &ds.ExtraParams,
		&ds.TunnelType, &ds.TunnelHost, &ds.TunnelPort, &ds.TunnelUser,
		&ds.TunnelPasswordEnc, &ds.TunnelPasswordIV, &ds.TunnelKeyEnc, &ds.TunnelKeyIV,
		&ds.TunnelKeyPassEnc, &ds.TunnelKeyPassIV, &ds.TunnelAuthScheme, &ds.TunnelTransport, &ds.TunnelUseSSL,
		&ds.Status, &lastTest, &ds.LastError, &ds.CreatedAt, &ds.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if lastTest.Valid {
		ds.LastTestAt = &lastTest.String
	}
	return &ds, nil
}

// DSFilter 数据源列表过滤条件。
type DSFilter struct {
	GroupID   int64
	DbType    string
	Status    int
	HasStatus bool
	Keyword   string
}

// DatasourceRepo 数据源仓储。
type DatasourceRepo struct {
	db *sql.DB
}

// NewDatasourceRepo 构造。
func NewDatasourceRepo(db *sql.DB) *DatasourceRepo { return &DatasourceRepo{db: db} }

// List 按条件查询。
func (r *DatasourceRepo) List(ctx context.Context, f DSFilter) ([]model.DataSource, error) {
	q := `SELECT ` + dsCols + ` FROM datasources WHERE 1=1`
	var args []any
	if f.GroupID > 0 {
		q += " AND group_id = ?"
		args = append(args, f.GroupID)
	}
	if f.DbType != "" {
		q += " AND db_type = ?"
		args = append(args, f.DbType)
	}
	if f.HasStatus {
		q += " AND status = ?"
		args = append(args, f.Status)
	}
	if f.Keyword != "" {
		q += " AND (name LIKE ? OR host LIKE ?)"
		kw := "%" + f.Keyword + "%"
		args = append(args, kw, kw)
	}
	q += " ORDER BY id DESC"

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.DataSource
	for rows.Next() {
		ds, err := scanDataSource(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *ds)
	}
	return out, rows.Err()
}

// Get 按 ID 查询。
func (r *DatasourceRepo) Get(ctx context.Context, id int64) (*model.DataSource, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+dsCols+` FROM datasources WHERE id = ?`, id)
	ds, err := scanDataSource(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return ds, nil
}

const dsInsertCols = `name, group_id, db_type, host, port, db_name, username,
	password_enc, password_iv, extra_params,
	tunnel_type, tunnel_host, tunnel_port, tunnel_user,
	tunnel_password_enc, tunnel_password_iv, tunnel_key_enc, tunnel_key_iv,
	tunnel_key_pass_enc, tunnel_key_pass_iv, tunnel_auth_scheme, tunnel_transport, tunnel_use_ssl`

func dsArgs(ds *model.DataSource) []any {
	return []any{
		ds.Name, ds.GroupID, ds.DbType, ds.Host, ds.Port, ds.DbName, ds.Username,
		ds.PasswordEnc, ds.PasswordIV, ds.ExtraParams,
		ds.TunnelType, ds.TunnelHost, ds.TunnelPort, ds.TunnelUser,
		ds.TunnelPasswordEnc, ds.TunnelPasswordIV, ds.TunnelKeyEnc, ds.TunnelKeyIV,
		ds.TunnelKeyPassEnc, ds.TunnelKeyPassIV, ds.TunnelAuthScheme, ds.TunnelTransport, ds.TunnelUseSSL,
	}
}

// Create 新建数据源。
func (r *DatasourceRepo) Create(ctx context.Context, ds *model.DataSource) error {
	q := `INSERT INTO datasources (` + dsInsertCols + `) VALUES (` + placeholders(23) + `)`
	res, err := r.db.ExecContext(ctx, q, dsArgs(ds)...)
	if err != nil {
		return err
	}
	ds.ID, err = res.LastInsertId()
	if err != nil {
		return err
	}
	return r.fillTimestamps(ctx, ds)
}

// Update 更新数据源。
func (r *DatasourceRepo) Update(ctx context.Context, ds *model.DataSource) error {
	q := `UPDATE datasources SET name=?, group_id=?, db_type=?, host=?, port=?, db_name=?, username=?,
	 password_enc=?, password_iv=?, extra_params=?,
	 tunnel_type=?, tunnel_host=?, tunnel_port=?, tunnel_user=?,
	 tunnel_password_enc=?, tunnel_password_iv=?, tunnel_key_enc=?, tunnel_key_iv=?,
	 tunnel_key_pass_enc=?, tunnel_key_pass_iv=?, tunnel_auth_scheme=?, tunnel_transport=?, tunnel_use_ssl=?,
	 updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE id = ?`
	args := append(dsArgs(ds), ds.ID)
	_, err := r.db.ExecContext(ctx, q, args...)
	if err != nil {
		return err
	}
	return r.fillTimestamps(ctx, ds)
}

func (r *DatasourceRepo) fillTimestamps(ctx context.Context, ds *model.DataSource) error {
	row := r.db.QueryRowContext(ctx, `SELECT created_at, updated_at FROM datasources WHERE id = ?`, ds.ID)
	return row.Scan(&ds.CreatedAt, &ds.UpdatedAt)
}

// Delete 删除数据源。
func (r *DatasourceRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM datasources WHERE id = ?`, id)
	return err
}

// UpdateTestResult 回写连通性测试结果。
func (r *DatasourceRepo) UpdateTestResult(ctx context.Context, id int64, status int, testedAt, lastErr string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE datasources SET status = ?, last_test_at = ?, last_error = ?,
		 updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE id = ?`,
		status, testedAt, lastErr, id)
	return err
}

// CountByGroup 统计分组下的数据源数量。
func (r *DatasourceRepo) CountByGroup(ctx context.Context, groupID int64) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM datasources WHERE group_id = ?`, groupID).Scan(&n)
	return n, err
}

// ListForExport 导出用查询（联表分组名，不含密码）。
func (r *DatasourceRepo) ListForExport(ctx context.Context) ([]model.ExportRow, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT d.db_type, d.name, d.host, d.port, d.db_name, d.username, COALESCE(g.name, ''),
		        d.tunnel_type, d.tunnel_host, d.tunnel_port, d.tunnel_user
		 FROM datasources d LEFT JOIN groups g ON g.id = d.group_id
		 ORDER BY d.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.ExportRow
	for rows.Next() {
		var e model.ExportRow
		if err := rows.Scan(&e.DbType, &e.Name, &e.Host, &e.Port, &e.DbName, &e.Username, &e.GroupName,
			&e.TunnelType, &e.TunnelHost, &e.TunnelPort, &e.TunnelUser); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func placeholders(n int) string {
	s := "?"
	for i := 1; i < n; i++ {
		s += ", ?"
	}
	return s
}
