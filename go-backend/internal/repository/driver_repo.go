package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"dbawork/internal/model"
)

const driverCols = `id, db_type, driver_kind, package_name, version, driver_class,
	jar_filename, jar_path, file_size, is_active, installed, installed_version, note,
	uploaded_at, updated_at`

// DriverRepo 驱动仓储。
type DriverRepo struct {
	db *sql.DB
}

// NewDriverRepo 构造。
func NewDriverRepo(db *sql.DB) *DriverRepo { return &DriverRepo{db: db} }

func scanDriver(row rowScanner) (*model.Driver, error) {
	var d model.Driver
	err := row.Scan(&d.ID, &d.DbType, &d.DriverKind, &d.PackageName, &d.Version, &d.DriverClass,
		&d.JarFilename, &d.JarPath, &d.FileSize, &d.IsActive, &d.Installed, &d.InstalledVersion, &d.Note,
		&d.UploadedAt, &d.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// List 查询驱动列表，dbType 为空时返回全部。
func (r *DriverRepo) List(ctx context.Context, dbType string) ([]model.Driver, error) {
	q := `SELECT ` + driverCols + ` FROM drivers`
	var args []any
	if dbType != "" {
		q += ` WHERE db_type = ?`
		args = append(args, dbType)
	}
	q += ` ORDER BY db_type, driver_kind, id`
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Driver
	for rows.Next() {
		d, err := scanDriver(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *d)
	}
	return out, rows.Err()
}

// GetByID 按 ID 查询。
func (r *DriverRepo) GetByID(ctx context.Context, id int64) (*model.Driver, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+driverCols+` FROM drivers WHERE id = ?`, id)
	d, err := scanDriver(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return d, nil
}

// Create 新建驱动记录。
func (r *DriverRepo) Create(ctx context.Context, d *model.Driver) error {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO drivers (db_type, driver_kind, package_name, version, driver_class,
		 jar_filename, jar_path, file_size, is_active, installed, installed_version, note)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		d.DbType, d.DriverKind, d.PackageName, d.Version, d.DriverClass,
		d.JarFilename, d.JarPath, d.FileSize, d.IsActive, d.Installed, d.InstalledVersion, d.Note)
	if err != nil {
		return err
	}
	d.ID, err = res.LastInsertId()
	return err
}

// EnsurePythonDriver 幂等写入内置 python 驱动行，并跟随类型目录迁移历史包名（detect 前确保存在）。
func (r *DriverRepo) EnsurePythonDriver(ctx context.Context, dbType, packageName string) error {
	// 历史包名迁移：同类型 python 行统一改为目录当前包名
	if _, err := r.db.ExecContext(ctx,
		`UPDATE drivers SET package_name = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now')
		 WHERE db_type = ? AND driver_kind = 'python' AND package_name <> ?`,
		packageName, dbType, packageName); err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			// 旧行与新行并存：清理未安装、未激活的历史行后重试
			if _, derr := r.db.ExecContext(ctx,
				`DELETE FROM drivers WHERE db_type = ? AND driver_kind = 'python'
				 AND package_name <> ? AND installed = 0 AND is_active = 0`,
				dbType, packageName); derr != nil {
				return derr
			}
			if _, rerr := r.db.ExecContext(ctx,
				`UPDATE drivers SET package_name = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now')
				 WHERE db_type = ? AND driver_kind = 'python' AND package_name <> ?`,
				packageName, dbType, packageName); rerr != nil {
				return rerr
			}
		} else {
			return err
		}
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO drivers (db_type, driver_kind, package_name, version, note)
		 VALUES (?, 'python', ?, '', '')`, dbType, packageName)
	return err
}

// Delete 删除驱动记录。
func (r *DriverRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM drivers WHERE id = ?`, id)
	return err
}

// UpdateRuntimeStatus 回写检测结果（按类型+种类批量更新）。
func (r *DriverRepo) UpdateRuntimeStatus(ctx context.Context, dbType, kind string, installed int, installedVersion, note string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE drivers SET installed = ?, installed_version = ?, note = ?,
		 updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now')
		 WHERE db_type = ? AND driver_kind = ?`,
		installed, installedVersion, note, dbType, kind)
	return err
}

// SetActive 设置某条驱动为对应 db_type 的默认驱动（事务内先清后置）。
func (r *DriverRepo) SetActive(ctx context.Context, id int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var dbType string
	if err := tx.QueryRowContext(ctx, `SELECT db_type FROM drivers WHERE id = ?`, id).Scan(&dbType); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("驱动不存在")
		}
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE drivers SET is_active = 0, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE db_type = ?`, dbType); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE drivers SET is_active = 1, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

// HasActive 判断某 db_type 是否已有默认驱动。
func (r *DriverRepo) HasActive(ctx context.Context, dbType string) (bool, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM drivers WHERE db_type = ? AND is_active = 1`, dbType).Scan(&n)
	return n > 0, err
}
