-- +goose Up
ALTER TABLE daily_sets ADD COLUMN round INT NOT NULL DEFAULT 0;
ALTER TABLE daily_sets DROP CONSTRAINT daily_sets_user_id_group_id_date_key;
ALTER TABLE daily_sets ADD CONSTRAINT daily_sets_user_id_group_id_date_round_key
    UNIQUE(user_id, group_id, date, round);

-- +goose Down
ALTER TABLE daily_sets DROP CONSTRAINT daily_sets_user_id_group_id_date_round_key;
ALTER TABLE daily_sets ADD CONSTRAINT daily_sets_user_id_group_id_date_key
    UNIQUE(user_id, group_id, date);
ALTER TABLE daily_sets DROP COLUMN round;
