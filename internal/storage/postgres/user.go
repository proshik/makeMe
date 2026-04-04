package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"makeMe/internal/domain/user"
	"makeMe/internal/storage/sqlc"
)

type UserRepository struct {
	q *sqlc.Queries
}

func NewUserRepository(q *sqlc.Queries) *UserRepository {
	return &UserRepository{q: q}
}

func (r *UserRepository) Create(ctx context.Context, telegramID int64, username, firstName string) (*user.User, error) {
	row, err := r.q.CreateUser(ctx, sqlc.CreateUserParams{
		TelegramID: telegramID,
		Username:   pgtype.Text{String: username, Valid: username != ""},
		FirstName:  firstName,
	})
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return toUser(row), nil
}

func (r *UserRepository) GetByTelegramID(ctx context.Context, telegramID int64) (*user.User, error) {
	row, err := r.q.GetUserByTelegramID(ctx, telegramID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, user.ErrNotFound
		}
		return nil, fmt.Errorf("get user by telegram id: %w", err)
	}
	return toUser(row), nil
}

func (r *UserRepository) GetByID(ctx context.Context, id int64) (*user.User, error) {
	row, err := r.q.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, user.ErrNotFound
		}
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return toUser(row), nil
}

func (r *UserRepository) UpdateLevel(ctx context.Context, id int64, level user.Level) error {
	err := r.q.UpdateUserLevel(ctx, sqlc.UpdateUserLevelParams{
		ID:    id,
		Level: string(level),
	})
	if err != nil {
		return fmt.Errorf("update user level: %w", err)
	}
	return nil
}

func toUser(row sqlc.User) *user.User {
	return &user.User{
		ID:         row.ID,
		TelegramID: row.TelegramID,
		Username:   row.Username.String,
		FirstName:  row.FirstName,
		Level:      user.Level(row.Level),
		Onboarded:  row.Onboarded,
		CreatedAt:  row.CreatedAt.Time,
		UpdatedAt:  row.UpdatedAt.Time,
	}
}
