package srs

import (
	"context"

	"makeMe/internal/domain/user"
)

type Repository interface {
	GetDueCards(ctx context.Context, userID int64, limit int) ([]ReviewCard, error)
	UpsertCard(ctx context.Context, card *ReviewCard) error
	GetWordsByLevel(ctx context.Context, level user.Level, limit int) ([]Word, error)
	GetWordByID(ctx context.Context, id int64) (*Word, error)
	GetNewWordsForUser(ctx context.Context, userID int64, level user.Level, limit int) ([]Word, error)
}
