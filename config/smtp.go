package config

// SMTPConfig defines the outbound SMTP connection used by mail-service.
type SMTPConfig struct {
	Host        string `env:"SMTP_HOST" envDefault:"smtp.gmail.com"`
	Port        int    `env:"SMTP_PORT" envDefault:"587"`
	Mode        string `env:"SMTP_MODE" envDefault:"starttls"`
	SenderEmail string `env:"SENDER_EMAIL,required"`
	Password    string `env:"EMAIL_PASSWORD,required"`
	FromName    string `env:"SMTP_FROM_NAME" envDefault:"OFM"`
}
