.PHONY: run build migrate-up migrate-down migrate-create sqlc seed-words install-tools

DATABASE_URL ?= postgres://makeme:makeme@localhost:5432/makeme?sslmode=disable

# Public Go module proxy — used only for tool installation in this project
GOPROXY_PUBLIC := https://proxy.golang.org,direct

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
	~/go/bin/sqlc generate

seed-words:
	go run ./cmd/seed data/words.json

# Install development tools using the public proxy (not the corporate one)
install-tools:
	GOPROXY=$(GOPROXY_PUBLIC) go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	GOPROXY=$(GOPROXY_PUBLIC) go install github.com/pressly/goose/v3/cmd/goose@latest
