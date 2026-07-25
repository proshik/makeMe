.PHONY: run build migrate-up migrate-down migrate-create sqlc install-tools css css-check

DATABASE_URL ?= postgres://makeme:makeme@localhost:5432/makeme?sslmode=disable

# Tailwind — отдельный standalone-бинарник, node не нужен.
TAILWIND_VERSION := v3.4.17
TAILWIND_BIN     := bin/tailwindcss
UNAME_S := $(shell uname -s)
UNAME_M := $(shell uname -m)
ifeq ($(UNAME_S),Darwin)
  TAILWIND_PLATFORM := macos-$(if $(filter arm64,$(UNAME_M)),arm64,x64)
else
  TAILWIND_PLATFORM := linux-$(if $(filter aarch64,$(UNAME_M)),arm64,x64)
endif

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

$(TAILWIND_BIN):
	@mkdir -p bin
	curl -sL -o $(TAILWIND_BIN) \
		https://github.com/tailwindlabs/tailwindcss/releases/download/$(TAILWIND_VERSION)/tailwindcss-$(TAILWIND_PLATFORM)
	chmod +x $(TAILWIND_BIN)

# Пересобрать CSS после правки шаблонов. Результат коммитится: //go:embed
# требует файл на месте во время компиляции, поэтому сборка Docker его не делает.
css: $(TAILWIND_BIN)
	$(TAILWIND_BIN) -c tailwind.config.js \
		-i internal/web/tailwind.src.css \
		-o internal/web/static/tailwind.css \
		--minify

# Проверка, что закоммиченный CSS соответствует шаблонам. Гоняется в CI.
# Через git status, а не git diff: на неотслеживаемом файле diff молчит,
# и проверка проходила бы вхолостую.
css-check: css
	@if [ -n "$$(git status --porcelain internal/web/static/tailwind.css)" ]; then \
		echo "internal/web/static/tailwind.css не соответствует шаблонам или не закоммичен —"; \
		echo "выполните 'make css' и закоммитьте результат"; \
		git status --short internal/web/static/tailwind.css; \
		exit 1; \
	fi

# Install development tools using the public proxy (not the corporate one)
install-tools:
	GOPROXY=$(GOPROXY_PUBLIC) go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	GOPROXY=$(GOPROXY_PUBLIC) go install github.com/pressly/goose/v3/cmd/goose@latest
