-- +goose Up
ALTER TABLE words ADD COLUMN transcription TEXT NOT NULL DEFAULT '';
ALTER TABLE words ADD COLUMN audio_url TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE words DROP COLUMN transcription;
ALTER TABLE words DROP COLUMN audio_url;
