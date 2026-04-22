package config

import "time"

// NATSConfig defines the mail-service command, result, and consumer settings.
type NATSConfig struct {
	URL                         string        `env:"NATS_URL,required"`
	User                        string        `env:"NATS_USER"`
	Password                    string        `env:"NATS_PASSWORD"`
	MailCommandsStream          string        `env:"NATS_STREAM_MAIL_COMMANDS" envDefault:"MAIL_COMMANDS"`
	MailEventsStream            string        `env:"NATS_STREAM_MAIL_EVENTS" envDefault:"MAIL_EVENTS"`
	MailSendSubject             string        `env:"NATS_SUBJECT_MAIL_SEND" envDefault:"mail.send"`
	MailSendResultSubject       string        `env:"NATS_SUBJECT_MAIL_SEND_RESULT" envDefault:"mail.send.result"`
	MailSendDurable             string        `env:"NATS_DURABLE_MAIL_SEND" envDefault:"mail_service_send"`
	MailBatchSize               int           `env:"NATS_MAIL_BATCH_SIZE" envDefault:"32"`
	MailMaxWait                 time.Duration `env:"NATS_MAIL_MAX_WAIT" envDefault:"10ms"`
	MailWorkers                 int           `env:"NATS_MAIL_WORKERS" envDefault:"8"`
	MailQueueSize               int           `env:"NATS_MAIL_QUEUE_SIZE" envDefault:"500"`
	MailAckWait                 time.Duration `env:"NATS_MAIL_ACK_WAIT" envDefault:"30s"`
	MailMaxDeliver              int           `env:"NATS_MAIL_MAX_DELIVER" envDefault:"5"`
	MailAdaptiveEnabled         bool          `env:"NATS_MAIL_ADAPTIVE_ENABLED" envDefault:"false"`
	MailAdaptiveCheckInterval   time.Duration `env:"NATS_MAIL_ADAPTIVE_CHECK_INTERVAL" envDefault:"2s"`
	MailAdaptiveMediumPending   int           `env:"NATS_MAIL_ADAPTIVE_MEDIUM_PENDING" envDefault:"200"`
	MailAdaptiveHighPending     int           `env:"NATS_MAIL_ADAPTIVE_HIGH_PENDING" envDefault:"1000"`
	MailAdaptiveLowBatchSize    int           `env:"NATS_MAIL_ADAPTIVE_LOW_BATCH_SIZE" envDefault:"8"`
	MailAdaptiveLowMaxWait      time.Duration `env:"NATS_MAIL_ADAPTIVE_LOW_MAX_WAIT" envDefault:"25ms"`
	MailAdaptiveMediumBatchSize int           `env:"NATS_MAIL_ADAPTIVE_MEDIUM_BATCH_SIZE" envDefault:"32"`
	MailAdaptiveMediumMaxWait   time.Duration `env:"NATS_MAIL_ADAPTIVE_MEDIUM_MAX_WAIT" envDefault:"10ms"`
	MailAdaptiveHighBatchSize   int           `env:"NATS_MAIL_ADAPTIVE_HIGH_BATCH_SIZE" envDefault:"128"`
	MailAdaptiveHighMaxWait     time.Duration `env:"NATS_MAIL_ADAPTIVE_HIGH_MAX_WAIT" envDefault:"2ms"`
}
