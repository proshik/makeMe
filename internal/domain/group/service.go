package group

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
)

var (
	ErrNotFound      = errors.New("group not found")
	ErrAlreadyMember = errors.New("user is already a member of this group")
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Create creates a new group and adds the creator as owner.
func (s *Service) Create(ctx context.Context, name string, createdBy int64) (*Group, error) {
	code, err := generateInviteCode()
	if err != nil {
		return nil, fmt.Errorf("generate invite code: %w", err)
	}

	g, err := s.repo.Create(ctx, name, code, createdBy)
	if err != nil {
		return nil, fmt.Errorf("create group: %w", err)
	}

	if err := s.repo.AddMember(ctx, g.ID, createdBy, RoleOwner); err != nil {
		return nil, fmt.Errorf("add owner to group: %w", err)
	}

	return g, nil
}

// Join adds a user to a group by invite code.
func (s *Service) Join(ctx context.Context, inviteCode string, userID int64) (*Group, error) {
	g, err := s.repo.GetByInviteCode(ctx, inviteCode)
	if err != nil {
		return nil, fmt.Errorf("get group by invite code: %w", err)
	}

	isMember, err := s.repo.IsMember(ctx, g.ID, userID)
	if err != nil {
		return nil, fmt.Errorf("check membership: %w", err)
	}
	if isMember {
		return nil, ErrAlreadyMember
	}

	if err := s.repo.AddMember(ctx, g.ID, userID, RoleMember); err != nil {
		return nil, fmt.Errorf("add member to group: %w", err)
	}

	return g, nil
}

// ListByUser returns all groups a user belongs to.
func (s *Service) ListByUser(ctx context.Context, userID int64) ([]Group, error) {
	groups, err := s.repo.ListByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list groups: %w", err)
	}
	return groups, nil
}

func generateInviteCode() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
