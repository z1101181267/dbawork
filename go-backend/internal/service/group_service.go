package service

import (
	"context"
	"errors"
	"strings"

	"dbawork/internal/model"
	"dbawork/internal/repository"
)

// GroupService 分组业务。
type GroupService struct {
	repo   *repository.GroupRepo
	dsRepo *repository.DatasourceRepo
}

// NewGroupService 构造。
func NewGroupService(repo *repository.GroupRepo, dsRepo *repository.DatasourceRepo) *GroupService {
	return &GroupService{repo: repo, dsRepo: dsRepo}
}

// Tree 返回嵌套分组树。
func (s *GroupService) Tree(ctx context.Context) ([]*model.Group, error) {
	list, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	nodes := map[int64]*model.Group{}
	for i := range list {
		g := list[i]
		g.Children = nil
		nodes[g.ID] = &g
	}
	roots := []*model.Group{}
	for i := range list {
		n := nodes[list[i].ID]
		if n.ParentID == 0 {
			roots = append(roots, n)
			continue
		}
		if p, ok := nodes[n.ParentID]; ok {
			p.Children = append(p.Children, n)
		} else {
			roots = append(roots, n)
		}
	}
	return roots, nil
}

// Create 新建分组。
func (s *GroupService) Create(ctx context.Context, g *model.Group) (*model.Group, error) {
	g.Name = strings.TrimSpace(g.Name)
	if g.Name == "" {
		return nil, errors.New("分组名称不能为空")
	}
	if g.ParentID != 0 {
		p, err := s.repo.Get(ctx, g.ParentID)
		if err != nil {
			return nil, err
		}
		if p == nil {
			return nil, errors.New("父分组不存在")
		}
	}
	if err := s.repo.Create(ctx, g); err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, g.ID)
}

// Update 更新分组。
func (s *GroupService) Update(ctx context.Context, g *model.Group) (*model.Group, error) {
	old, err := s.repo.Get(ctx, g.ID)
	if err != nil {
		return nil, err
	}
	if old == nil {
		return nil, errors.New("分组不存在")
	}
	g.Name = strings.TrimSpace(g.Name)
	if g.Name == "" {
		return nil, errors.New("分组名称不能为空")
	}
	if g.ParentID != 0 {
		if g.ParentID == g.ID {
			return nil, errors.New("不能将分组移动到自身")
		}
		p, err := s.repo.Get(ctx, g.ParentID)
		if err != nil {
			return nil, err
		}
		if p == nil {
			return nil, errors.New("父分组不存在")
		}
		desc, err := s.repo.DescendantIDs(ctx, g.ID)
		if err != nil {
			return nil, err
		}
		for _, d := range desc {
			if d == g.ParentID {
				return nil, errors.New("不能将分组移动到其子分组下")
			}
		}
	}
	if err := s.repo.Update(ctx, g); err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, g.ID)
}

// Delete 删除分组（需先清空子分组与数据源）。
func (s *GroupService) Delete(ctx context.Context, id int64) error {
	old, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if old == nil {
		return errors.New("分组不存在")
	}
	n, err := s.repo.CountChildren(ctx, id)
	if err != nil {
		return err
	}
	if n > 0 {
		return repository.ErrGroupHasContent
	}
	dsCount, err := s.dsRepo.CountByGroup(ctx, id)
	if err != nil {
		return err
	}
	if dsCount > 0 {
		return repository.ErrGroupHasContent
	}
	return s.repo.Delete(ctx, id)
}
