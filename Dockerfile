# Builder pinned to the same Alpine branch as the runtime image, so both stages
# move together when the branch is bumped.
#
# The builder runs on the build host's own platform and cross-compiles to the
# target one. Without that, `--platform linux/amd64` on an arm64 machine runs
# the Go toolchain under qemu, which crashes in `go mod download`
# ("fatal error: runtime: name offset base pointer out of range"). On an amd64
# CI runner the two platforms coincide and nothing changes.
FROM --platform=$BUILDPLATFORM golang:1.26-alpine3.24 AS builder
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# internal/web/static (including the pre-built tailwind.css) is compiled in via
# //go:embed, so no CSS step runs here — the committed file is what ships.
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/makeme ./cmd/makeme

FROM alpine:3.24
RUN apk add --no-cache ca-certificates tzdata \
	&& addgroup -S -g 10001 makeme \
	&& adduser -S -D -H -u 10001 -G makeme -s /sbin/nologin makeme
# The binary resolves migrations/ (goose.Up(db, "migrations")) and the seed
# files (WORDS_FILE etc., default data/*.json) relative to the working
# directory, so both must sit directly under WORKDIR.
WORKDIR /app
COPY --from=builder /out/makeme /makeme
COPY migrations /app/migrations
COPY data /app/data
# Numeric UID so an orchestrator enforcing runAsNonRoot can verify it. The app
# only reads from disk; the copied files stay root-owned and read-only to it.
USER 10001:10001
EXPOSE 8080
CMD ["/makeme"]
