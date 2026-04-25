package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"makeMe/internal/domain/words"
	"makeMe/internal/storage/sqlc"
)

type PendingTypingRepository struct {
	q *sqlc.Queries
}

func NewPendingTypingRepository(q *sqlc.Queries) *PendingTypingRepository {
	return &PendingTypingRepository{q: q}
}

func (r *PendingTypingRepository) Upsert(ctx context.Context, p *words.PendingTyping) error {
	payload, err := json.Marshal(p.Payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	return r.q.UpsertPendingTyping(ctx, sqlc.UpsertPendingTypingParams{
		UserID:    p.UserID,
		ChatID:    p.ChatID,
		Source:    p.Source,
		Payload:   payload,
		ExpiresAt: pgtype.Timestamptz{Time: p.ExpiresAt, Valid: true},
	})
}

func (r *PendingTypingRepository) Get(ctx context.Context, userID int64) (*words.PendingTyping, error) {
	row, err := r.q.GetPendingTyping(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get pending typing: %w", err)
	}
	var payload words.PendingPayload
	if len(row.Payload) > 0 {
		if err := json.Unmarshal(row.Payload, &payload); err != nil {
			return nil, fmt.Errorf("unmarshal payload: %w", err)
		}
	}
	return &words.PendingTyping{
		UserID:    row.UserID,
		ChatID:    row.ChatID,
		Source:    row.Source,
		Payload:   payload,
		ExpiresAt: row.ExpiresAt.Time,
	}, nil
}

func (r *PendingTypingRepository) Delete(ctx context.Context, userID int64) error {
	return r.q.DeletePendingTyping(ctx, userID)
}
