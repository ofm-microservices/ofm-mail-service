package smtp

import (
	"context"
	"github.com/ofm-microseervices/ofm-common/pkg/logging"
	"mail-service/config"
	mail "mail-service/internal/domain"
)

// Sender is the low-level mail transport contract used by the application
// layer.
type Sender interface {
	Send(ctx context.Context, msg mail.Email) error
}

// Logger aliases the shared logger contract used by the SMTP adapter.
type Logger = logging.Logger

// SMTPConfig aliases the SMTP configuration owned by mail-service.
type SMTPConfig = config.SMTPConfig
