package user

import "context"

type Repository interface {
	Create(ctx context.Context, telegramID int64, username, firstName string) (*User, error)
	GetByTelegramID(ctx context.Context, telegramID int64) (*User, error)
	GetByID(ctx context.Context, id int64) (*User, error)
	UpdateLevel(ctx context.Context, id int64, level Level) error
	ListAll(ctx context.Context) ([]User, error)
}
