package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"makeMe/internal/domain/srs"
	"makeMe/internal/domain/user"
	"makeMe/internal/storage/sqlc"
)

type SRSRepository struct {
	q *sqlc.Queries
}

func NewSRSRepository(q *sqlc.Queries) *SRSRepository {
	return &SRSRepository{q: q}
}

func (r *SRSRepository) GetDueCards(ctx context.Context, userID int64, limit int) ([]srs.ReviewCard, error) {
	rows, err := r.q.GetDueCards(ctx, sqlc.GetDueCardsParams{
		UserID: userID,
		Limit:  int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("get due cards: %w", err)
	}

	cards := make([]srs.ReviewCard, len(rows))
	for i, row := range rows {
		cards[i] = toReviewCard(row)
	}
	return cards, nil
}

func (r *SRSRepository) UpsertCard(ctx context.Context, card *srs.ReviewCard) error {
	err := r.q.UpsertReviewCard(ctx, sqlc.UpsertReviewCardParams{
		UserID:       card.UserID,
		WordID:       card.WordID,
		EaseFactor:   float32(card.EaseFactor),
		IntervalDays: int32(card.IntervalDays),
		Repetitions:  int32(card.Repetitions),
		NextReviewDate: pgtype.Date{
			Time:  card.NextReviewDate,
			Valid: true,
		},
	})
	if err != nil {
		return fmt.Errorf("upsert review card: %w", err)
	}
	return nil
}

func (r *SRSRepository) GetWordsByLevel(ctx context.Context, level user.Level, limit int) ([]srs.Word, error) {
	rows, err := r.q.GetWordsByLevel(ctx, sqlc.GetWordsByLevelParams{
		Level: string(level),
		Limit: int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("get words by level: %w", err)
	}

	words := make([]srs.Word, len(rows))
	for i, row := range rows {
		words[i] = toWord(row)
	}
	return words, nil
}

func (r *SRSRepository) GetWordByID(ctx context.Context, id int64) (*srs.Word, error) {
	row, err := r.q.GetWordByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("word not found")
		}
		return nil, fmt.Errorf("get word by id: %w", err)
	}
	w := toWord(row)
	return &w, nil
}

func (r *SRSRepository) GetNewWordsForUser(ctx context.Context, userID int64, level user.Level, limit int) ([]srs.Word, error) {
	rows, err := r.q.GetNewWordsForUser(ctx, sqlc.GetNewWordsForUserParams{
		UserID: userID,
		Level:  string(level),
		Limit:  int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("get new words: %w", err)
	}

	words := make([]srs.Word, len(rows))
	for i, row := range rows {
		words[i] = toWord(row)
	}
	return words, nil
}

func toReviewCard(row sqlc.ReviewCard) srs.ReviewCard {
	return srs.ReviewCard{
		ID:             row.ID,
		UserID:         row.UserID,
		WordID:         row.WordID,
		EaseFactor:     float64(row.EaseFactor),
		IntervalDays:   int(row.IntervalDays),
		Repetitions:    int(row.Repetitions),
		NextReviewDate: row.NextReviewDate.Time,
		UpdatedAt:      row.UpdatedAt.Time,
	}
}

func (r *SRSRepository) UpdateWordAudio(ctx context.Context, wordID int64, transcription, audioURL string) error {
	err := r.q.UpdateWordAudio(ctx, sqlc.UpdateWordAudioParams{
		ID:            wordID,
		Transcription: transcription,
		AudioUrl:      audioURL,
	})
	if err != nil {
		return fmt.Errorf("update word audio: %w", err)
	}
	return nil
}

func toWord(row sqlc.Word) srs.Word {
	return srs.Word{
		ID:            row.ID,
		English:       row.English,
		Russian:       row.Russian,
		Level:         user.Level(row.Level),
		Example:       row.Example,
		Transcription: row.Transcription,
		AudioURL:      row.AudioUrl,
	}
}

func (r *SRSRepository) CountLearnedWords(ctx context.Context, userID int64) (int, error) {
	count, err := r.q.CountLearnedWords(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("count learned words: %w", err)
	}
	return int(count), nil
}
