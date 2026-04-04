package group

import "time"

type MemberRole string

const (
	RoleOwner  MemberRole = "owner"
	RoleMember MemberRole = "member"
)

type Group struct {
	ID         int64
	Name       string
	InviteCode string
	CreatedBy  int64
	CreatedAt  time.Time
}

type Membership struct {
	ID       int64
	GroupID  int64
	UserID   int64
	Role     MemberRole
	JoinedAt time.Time
}
