package nats

import (
	"github.com/ofm-microservices/ofm-common/pkg/logging"
	"mail-service/config"
	"time"

	"github.com/nats-io/nats.go"
)

// EnsureStream creates or updates the JetStream streams required by
// mail-service.
func EnsureStream(cfg config.NATSConfig, log logging.Logger) error {
	if log == nil {
		return ErrNilLogger
	}

	lg := log.With(logging.String("module", "jetstream-bootstrap"))
	lg.Info("ensuring jetstream streams",
		logging.String("mail_commands_stream", cfg.MailCommandsStream),
		logging.String("mail_events_stream", cfg.MailEventsStream),
	)

	nc, err := Connect(cfg)
	if err != nil {
		return err
	}
	defer nc.Close()

	js, err := nc.JetStream()
	if err != nil {
		return WrapInitJetStreamContextError(err)
	}

	mailCommandsStream := &nats.StreamConfig{
		Name:      cfg.MailCommandsStream,
		Subjects:  []string{cfg.MailSendSubject},
		Storage:   nats.FileStorage,
		Retention: nats.LimitsPolicy,
		Replicas:  1,
		MaxAge:    7 * 24 * time.Hour,
	}

	if _, err := js.AddStream(mailCommandsStream); err != nil {
		if _, updateErr := js.UpdateStream(mailCommandsStream); updateErr != nil {
			return WrapEnsureStreamError(mailCommandsStream.Name, err, updateErr)
		}
	}

	mailEventsStream := &nats.StreamConfig{
		Name:      cfg.MailEventsStream,
		Subjects:  []string{cfg.MailSendResultSubject},
		Storage:   nats.FileStorage,
		Retention: nats.LimitsPolicy,
		Replicas:  1,
		MaxAge:    7 * 24 * time.Hour,
	}

	if _, err := js.AddStream(mailEventsStream); err != nil {
		if _, updateErr := js.UpdateStream(mailEventsStream); updateErr != nil {
			return WrapEnsureStreamError(mailEventsStream.Name, err, updateErr)
		}
	}

	lg.Info("jetstream streams ensured")
	return nil
}

// Connect establishes the low-level NATS connection used by bootstrap code.
func Connect(cfg config.NATSConfig) (*nats.Conn, error) {
	opts := []nats.Option{
		nats.Name("mail-service"),
		nats.MaxReconnects(-1),
	}

	if cfg.User != "" {
		opts = append(opts, nats.UserInfo(cfg.User, cfg.Password))
	}

	nc, err := nats.Connect(cfg.URL, opts...)
	if err != nil {
		return nil, WrapConnectToNATSError(err)
	}

	return nc, nil
}
