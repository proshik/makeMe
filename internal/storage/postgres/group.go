package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"makeMe/internal/domain/group"
	"makeMe/internal/storage/sqlc"
)

type GroupRepository struct {
	q *sqlc.Queries
}

func NewGroupRepository(q *sqlc.Queries) *GroupRepository {
	return &GroupRepository{q: q}
}

func (r *GroupRepository) Create(ctx context.Context, name, inviteCode string, createdBy int64) (*group.Group, error) {
	row, err := r.q.CreateGroup(ctx, sqlc.CreateGroupParams{
		Name:       name,
		InviteCode: inviteCode,
		CreatedBy:  createdBy,
	})
	if err != nil {
		return nil, fmt.Errorf("create group: %w", err)
	}
	return toGroup(row), nil
}

func (r *GroupRepository) GetByInviteCode(ctx context.Context, inviteCode string) (*group.Group, error) {
	row, err := r.q.GetGroupByInviteCode(ctx, inviteCode)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, group.ErrNotFound
		}
		return nil, fmt.Errorf("get group by invite code: %w", err)
	}
	return toGroup(row), nil
}

func (r *GroupRepository) GetByID(ctx context.Context, id int64) (*group.Group, error) {
	row, err := r.q.GetGroupByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, group.ErrNotFound
		}
		return nil, fmt.Errorf("get group by id: %w", err)
	}
	return toGroup(row), nil
}

func (r *GroupRepository) AddMember(ctx context.Context, groupID, userID int64, role group.MemberRole) error {
	err := r.q.AddMembership(ctx, sqlc.AddMembershipParams{
		GroupID: groupID,
		UserID:  userID,
		Role:    string(role),
	})
	if err != nil {
		return fmt.Errorf("add membership: %w", err)
	}
	return nil
}

func (r *GroupRepository) ListByUser(ctx context.Context, userID int64) ([]group.Group, error) {
	rows, err := r.q.ListGroupsByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list groups by user: %w", err)
	}
	groups := make([]group.Group, len(rows))
	for i, row := range rows {
		groups[i] = *toGroup(row)
	}
	return groups, nil
}

func (r *GroupRepository) IsMember(ctx context.Context, groupID, userID int64) (bool, error) {
	is, err := r.q.IsMember(ctx, sqlc.IsMemberParams{
		GroupID: groupID,
		UserID:  userID,
	})
	if err != nil {
		return false, fmt.Errorf("check membership: %w", err)
	}
	return is, nil
}

func toGroup(row sqlc.Group) *group.Group {
	return &group.Group{
		ID:         row.ID,
		Name:       row.Name,
		InviteCode: row.InviteCode,
		CreatedBy:  row.CreatedBy,
		CreatedAt:  row.CreatedAt.Time,
	}
}
