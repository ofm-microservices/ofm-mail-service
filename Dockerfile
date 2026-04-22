FROM golang:1.25.5 AS builder

WORKDIR /src

COPY ofm-common /src/ofm-common
COPY ofm-mail-service /src/ofm-mail-service

WORKDIR /src/ofm-mail-service

RUN go mod download

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/mail-service ./cmd/mail-service

FROM debian:bookworm-slim

WORKDIR /app

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && groupadd --system app \
    && useradd --system --gid app --home-dir /app --shell /usr/sbin/nologin app \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /out/mail-service /app/mail-service
COPY --from=builder /src/ofm-mail-service/templates /app/templates

RUN chown -R app:app /app

USER app:app

CMD ["/app/mail-service"]
