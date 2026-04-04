package group

import "context"

type Repository interface {
	Create(ctx context.Context, name, inviteCode string, createdBy int64) (*Group, error)
	GetByInviteCode(ctx context.Context, inviteCode string) (*Group, error)
	GetByID(ctx context.Context, id int64) (*Group, error)
	AddMember(ctx context.Context, groupID, userID int64, role MemberRole) error
	ListByUser(ctx context.Context, userID int64) ([]Group, error)
	IsMember(ctx context.Context, groupID, userID int64) (bool, error)
}
