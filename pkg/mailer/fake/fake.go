// Package fake provides a no-op mail transport for local experiments.
package fake

import (
	"context"
	mail "mail-service/internal/domain"
)

// Sender accepts mail without contacting an external SMTP provider. The
// application still emits its normal success result, so registration flows
// remain fully exercised without serial SMTP latency.
type Sender struct{}

// Send implements the mail transport contract for experiment mode.
func (Sender) Send(context.Context, mail.Email) error { return nil }
