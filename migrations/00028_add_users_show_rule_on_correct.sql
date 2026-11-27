-- +goose Up
-- Web setting: show the grammar rule after a correct answer too. After a wrong
-- answer the rule is always shown, so the default is off.
ALTER TABLE users
    ADD COLUMN show_rule_on_correct BOOLEAN NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE users
    DROP COLUMN show_rule_on_correct;
