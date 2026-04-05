.PHONY: run build migrate-up migrate-down migrate-create sqlc seed-words

DATABASE_URL ?= postgres://makeme:makeme@localhost:5432/makeme?sslmode=disable

run:
	go run ./cmd/makeme

build:
	go build -o bin/makeme ./cmd/makeme

migrate-up:
	goose -dir migrations postgres "$(DATABASE_URL)" up

migrate-down:
	goose -dir migrations postgres "$(DATABASE_URL)" down

migrate-create:
	goose -dir migrations create $(name) sql

sqlc:
	sqlc generate

seed-words:
	go run ./cmd/seed data/words.json
