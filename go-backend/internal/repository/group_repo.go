// Package repository 数据访问层。
package repository

import (
	"context"
	"database/sql"
	"errors"

	"dbawork/internal/model"
)

// GroupRepo 分组仓储。
type GroupRepo struct {
	db *sql.DB
}

// NewGroupRepo 构造。
func NewGroupRepo(db *sql.DB) *GroupRepo { return &GroupRepo{db: db} }

// List 返回全部分组（按 sort_order, id 排序）。
func (r *GroupRepo) List(ctx context.Context) ([]model.Group, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, parent_id, sort_order, description, created_at, updated_at
		 FROM groups ORDER BY sort_order, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Group
	for rows.Next() {
		var g model.Group
		if err := rows.Scan(&g.ID, &g.Name, &g.ParentID, &g.SortOrder, &g.Description, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// Get 按 ID 查询。
func (r *GroupRepo) Get(ctx context.Context, id int64) (*model.Group, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, name, parent_id, sort_order, description, created_at, updated_at
		 FROM groups WHERE id = ?`, id)
	var g model.Group
	if err := row.Scan(&g.ID, &g.Name, &g.ParentID, &g.SortOrder, &g.Description, &g.CreatedAt, &g.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &g, nil
}

// FindByName 按名称精确查找（CSV 导入用）。
func (r *GroupRepo) FindByName(ctx context.Context, name string) (*model.Group, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, name, parent_id, sort_order, description, created_at, updated_at
		 FROM groups WHERE name = ? ORDER BY id LIMIT 1`, name)
	var g model.Group
	if err := row.Scan(&g.ID, &g.Name, &g.ParentID, &g.SortOrder, &g.Description, &g.CreatedAt, &g.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &g, nil
}

// Create 新建分组。
func (r *GroupRepo) Create(ctx context.Context, g *model.Group) error {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO groups (name, parent_id, sort_order, description) VALUES (?, ?, ?, ?)`,
		g.Name, g.ParentID, g.SortOrder, g.Description)
	if err != nil {
		return err
	}
	g.ID, err = res.LastInsertId()
	return err
}

// Update 更新分组。
func (r *GroupRepo) Update(ctx context.Context, g *model.Group) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE groups SET name = ?, parent_id = ?, sort_order = ?, description = ?,
		 updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE id = ?`,
		g.Name, g.ParentID, g.SortOrder, g.Description, g.ID)
	return err
}

// Delete 删除分组。
func (r *GroupRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM groups WHERE id = ?`, id)
	return err
}

// CountChildren 统计直接子分组数量。
func (r *GroupRepo) CountChildren(ctx context.Context, parentID int64) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM groups WHERE parent_id = ?`, parentID).Scan(&n)
	return n, err
}

// DescendantIDs 返回该分组的全部后代 ID（含自身）。
func (r *GroupRepo) DescendantIDs(ctx context.Context, id int64) ([]int64, error) {
	all, err := r.List(ctx)
	if err != nil {
		return nil, err
	}
	children := map[int64][]int64{}
	for _, g := range all {
		children[g.ParentID] = append(children[g.ParentID], g.ID)
	}
	var out []int64
	stack := []int64{id}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		out = append(out, cur)
		stack = append(stack, children[cur]...)
	}
	return out, nil
}

// ErrGroupHasContent 分组下仍有子分组或数据源。
var ErrGroupHasContent = errors.New("分组下仍有子分组或数据源")
