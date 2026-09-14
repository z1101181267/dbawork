package repository

import (
	"context"
	"database/sql"

	"dbawork/internal/model"
)

// DbTypeRepo 数据库类型目录（隐藏/自定义）仓储。
type DbTypeRepo struct {
	db *sql.DB
}

// NewDbTypeRepo 构造。
func NewDbTypeRepo(db *sql.DB) *DbTypeRepo { return &DbTypeRepo{db: db} }

// HiddenList 返回全部被隐藏的 db_type。
func (r *DbTypeRepo) HiddenList(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT db_type FROM db_type_hidden ORDER BY db_type`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// IsHidden 判断是否隐藏。
func (r *DbTypeRepo) IsHidden(ctx context.Context, key string) (bool, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM db_type_hidden WHERE db_type = ?`, key).Scan(&n)
	return n > 0, err
}

// Hide 隐藏内置类型（幂等）。
func (r *DbTypeRepo) Hide(ctx context.Context, key string) error {
	_, err := r.db.ExecContext(ctx, `INSERT OR IGNORE INTO db_type_hidden (db_type) VALUES (?)`, key)
	return err
}

// Restore 恢复隐藏（幂等）。
func (r *DbTypeRepo) Restore(ctx context.Context, key string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM db_type_hidden WHERE db_type = ?`, key)
	return err
}

// CustomList 返回全部自定义类型。
func (r *DbTypeRepo) CustomList(ctx context.Context) ([]model.DbTypeCustom, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT db_type, name_zh, name_en, driver_class_hint, is_jdbc, created_at FROM db_type_custom ORDER BY db_type`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.DbTypeCustom
	for rows.Next() {
		var c model.DbTypeCustom
		if err := rows.Scan(&c.DbType, &c.NameZh, &c.NameEn, &c.DriverClassHint, &c.IsJdbc, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// CustomExists 判断自定义类型是否存在。
func (r *DbTypeRepo) CustomExists(ctx context.Context, key string) (bool, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM db_type_custom WHERE db_type = ?`, key).Scan(&n)
	return n > 0, err
}

// CreateCustom 新建自定义类型。
func (r *DbTypeRepo) CreateCustom(ctx context.Context, c *model.DbTypeCustom) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO db_type_custom (db_type, name_zh, name_en, driver_class_hint, is_jdbc) VALUES (?, ?, ?, ?, ?)`,
		c.DbType, c.NameZh, c.NameEn, c.DriverClassHint, c.IsJdbc)
	return err
}

// DeleteCustom 删除自定义类型。
func (r *DbTypeRepo) DeleteCustom(ctx context.Context, key string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM db_type_custom WHERE db_type = ?`, key)
	return err
}
