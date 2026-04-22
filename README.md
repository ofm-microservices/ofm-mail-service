# OFM Mail Service

## Purpose

`ofm-mail-service` is the outbound email delivery service for OFM. It consumes
mail commands from NATS, renders registered HTML templates, sends messages over
SMTP, and publishes per-message results back to the event bus.

Current responsibilities:

- subscribe to `mail.send`
- render message templates by `message_type`
- send HTML email via SMTP
- publish `mail.send.result`

The service does not own registration logic, auth data, or notification read
models. It is a delivery worker.

## Run

Local process:

```bash
cp .env.example .env
go run ./cmd/mail-service
```

Docker stack from the shared infra repo:

```bash
cd ../ofm-infra
just infra-up
```

## Environment

```env
APP_ENV=local
LOG_LEVEL=info

SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_MODE=starttls
SENDER_EMAIL=your-email@gmail.com
EMAIL_PASSWORD=your-app-password
SMTP_FROM_NAME=OFM

MAIL_TEMPLATE_DIR=templates

NATS_URL=nats://127.0.0.1:4222
NATS_USER=
NATS_PASSWORD=
NATS_STREAM_MAIL_COMMANDS=MAIL_COMMANDS
NATS_STREAM_MAIL_EVENTS=MAIL_EVENTS
NATS_SUBJECT_MAIL_SEND=mail.send
NATS_SUBJECT_MAIL_SEND_RESULT=mail.send.result
NATS_DURABLE_MAIL_SEND=mail_service_send
NATS_MAIL_BATCH_SIZE=32
NATS_MAIL_MAX_WAIT=10ms
NATS_MAIL_WORKERS=8
NATS_MAIL_QUEUE_SIZE=500
NATS_MAIL_ACK_WAIT=30s
NATS_MAIL_MAX_DELIVER=5
NATS_MAIL_ADAPTIVE_ENABLED=false
NATS_MAIL_ADAPTIVE_CHECK_INTERVAL=2s
NATS_MAIL_ADAPTIVE_MEDIUM_PENDING=200
NATS_MAIL_ADAPTIVE_HIGH_PENDING=1000
NATS_MAIL_ADAPTIVE_LOW_BATCH_SIZE=8
NATS_MAIL_ADAPTIVE_LOW_MAX_WAIT=25ms
NATS_MAIL_ADAPTIVE_MEDIUM_BATCH_SIZE=32
NATS_MAIL_ADAPTIVE_MEDIUM_MAX_WAIT=10ms
NATS_MAIL_ADAPTIVE_HIGH_BATCH_SIZE=128
NATS_MAIL_ADAPTIVE_HIGH_MAX_WAIT=2ms
```

Operational note:

- for Gmail, `EMAIL_PASSWORD` should normally be an app password, not the
  account password

## Technologies

Core runtime:

- Go
- NATS JetStream for command/result messaging
- SMTP for delivery
- filesystem HTML templates
- Uber Fx for wiring
- Zap for structured logging

Main libraries from `go.mod`:

- `github.com/nats-io/nats.go`
- `go.uber.org/fx`
- `go.uber.org/zap`
- `github.com/caarlos0/env/v11`
- `github.com/joho/godotenv`

## Architecture Notes

- `internal/domain` defines mail request/result contracts and domain errors
- `internal/application` validates requests and renders templates
- `internal/presentation/event_broker/nats` owns broker subscription logic
- `pkg/mailer/smtp` owns the SMTP transport adapter
- `templates/` contains HTML templates registered by message type

To add a new message type:

1. add a template file under `templates/`
2. register a `TemplateDefinition` in `internal/fx/service.go`
3. publish a `mail.send` command with matching `message_type`
