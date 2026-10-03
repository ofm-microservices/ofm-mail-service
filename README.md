# OFM Mail Service

## Purpose

The Mail Service is an outbound email worker. It consumes mail commands, renders registered HTML templates, sends messages through SMTP, and publishes per-message results. It does not own registration, auth, or notification state. Status: active.

## Interfaces and flow

NATS delivers mail.send commands. The service validates the message type, renders the matching template from templates/, sends it through SMTP, and publishes mail.send.result. Durable consumers, retry limits, and result publication are part of the delivery boundary.

## Configuration

.env.example groups are SMTP_* and sender credentials, template path, NATS_*, health settings, and observability. SMTP values select the server and mode; sender values identify the outbound address; NATS values select the command stream and durable consumer. Gmail normally requires an app password.

## Local development

    cp .env.example .env
    go run ./cmd/mail-service
    go test ./...

## Build and operations

Dockerfile builds ofm/mail-service:<tag>. ofm-infra deploys the worker and supplies NATS and observability. Diagnose template registration, SMTP responses, NATS redeliveries, result events, and trace IDs.

