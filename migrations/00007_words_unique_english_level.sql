-- +goose Up
CREATE UNIQUE INDEX idx_words_english_level ON words(english, level);

-- +goose Down
DROP INDEX idx_words_english_level;
