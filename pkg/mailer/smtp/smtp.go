package smtp

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/ofm-microseervices/ofm-common/pkg/logging"
	mail "mail-service/internal/domain"
	"net"
	netmail "net/mail"
	"net/smtp"
	"strings"
)

const sendMailWrapFormat = "%w: %v"

var newTLSConfig = func(host string) *tls.Config {
	return &tls.Config{
		ServerName: host,
	}
}

type smtpSender struct {
	cfg      SMTPConfig
	from     string
	fromAddr string
	log      logging.Logger
}

// New constructs the SMTP sender used by mail-service.
func New(cfg SMTPConfig, log Logger) (Sender, error) {
	if strings.TrimSpace(cfg.Host) == "" {
		return nil, ErrEmptyHost
	}
	if cfg.Port <= 0 {
		return nil, ErrInvalidPort
	}
	if strings.TrimSpace(cfg.SenderEmail) == "" {
		return nil, ErrEmptySenderEmail
	}
	if strings.TrimSpace(cfg.Password) == "" {
		return nil, ErrEmptyPassword
	}
	if log == nil {
		return nil, ErrNilLogger
	}
	if cfg.Mode != "starttls" && cfg.Mode != "tls" {
		return nil, ErrInvalidMode
	}

	from := cfg.SenderEmail
	if strings.TrimSpace(cfg.FromName) != "" {
		from = (&netmail.Address{Name: cfg.FromName, Address: cfg.SenderEmail}).String()
	}

	return &smtpSender{
		cfg:      cfg,
		from:     from,
		fromAddr: cfg.SenderEmail,
		log:      log.With(logging.String("module", "smtp-sender")),
	}, nil
}

// Send validates the outbound email and delivers it over SMTP.
func (s *smtpSender) Send(ctx context.Context, msg mail.Email) error {
	if _, err := netmail.ParseAddress(msg.To); err != nil {
		return mail.ErrInvalidRecipientEmail
	}

	client, conn, err := s.newClient(ctx)
	if err != nil {
		return fmt.Errorf(sendMailWrapFormat, mail.ErrFailedToSendMail, err)
	}
	defer conn.Close()
	defer client.Close()

	if err := s.authenticate(client); err != nil {
		return fmt.Errorf(sendMailWrapFormat, mail.ErrFailedToSendMail, err)
	}
	if err := client.Mail(s.fromAddr); err != nil {
		return fmt.Errorf(sendMailWrapFormat, mail.ErrFailedToSendMail, WrapSMTPMailFromError(err))
	}
	if err := client.Rcpt(msg.To); err != nil {
		return fmt.Errorf(sendMailWrapFormat, mail.ErrFailedToSendMail, WrapSMTPRcptToError(err))
	}

	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf(sendMailWrapFormat, mail.ErrFailedToSendMail, WrapSMTPDataError(err))
	}

	if _, err := writer.Write([]byte(s.buildMessage(msg))); err != nil {
		_ = writer.Close()
		return fmt.Errorf(sendMailWrapFormat, mail.ErrFailedToSendMail, WrapSMTPWriteError(err))
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf(sendMailWrapFormat, mail.ErrFailedToSendMail, WrapSMTPWriteError(err))
	}

	if err := client.Quit(); err != nil {
		s.log.Warn("smtp quit failed", logging.Err(err))
	}

	s.log.Info("smtp message sent", logging.String("to", msg.To), logging.String("subject", msg.Subject))
	return nil
}

func (s *smtpSender) newClient(ctx context.Context) (*smtp.Client, net.Conn, error) {
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	dialer := &net.Dialer{}

	switch s.cfg.Mode {
	case "tls":
		tlsConn, err := tls.DialWithDialer(dialer, "tcp", addr, newTLSConfig(s.cfg.Host))
		if err != nil {
			return nil, nil, WrapDialSMTPError(err)
		}
		client, err := smtp.NewClient(tlsConn, s.cfg.Host)
		if err != nil {
			tlsConn.Close()
			return nil, nil, WrapCreateSMTPClientError(err)
		}
		return client, tlsConn, nil
	default:
		conn, err := dialer.DialContext(ctx, "tcp", addr)
		if err != nil {
			return nil, nil, WrapDialSMTPError(err)
		}
		client, err := smtp.NewClient(conn, s.cfg.Host)
		if err != nil {
			conn.Close()
			return nil, nil, WrapCreateSMTPClientError(err)
		}
		if s.cfg.Mode == "starttls" {
			if err := client.StartTLS(newTLSConfig(s.cfg.Host)); err != nil {
				conn.Close()
				return nil, nil, WrapStartTLSError(err)
			}
		}
		return client, conn, nil
	}
}

func (s *smtpSender) authenticate(client *smtp.Client) error {
	auth := smtp.PlainAuth("", s.cfg.SenderEmail, s.cfg.Password, s.cfg.Host)
	if err := client.Auth(auth); err != nil {
		return WrapSMTPAuthError(err)
	}
	return nil
}

func (s *smtpSender) buildMessage(msg mail.Email) string {
	headers := []string{
		fmt.Sprintf("From: %s", s.from),
		fmt.Sprintf("To: %s", msg.To),
		fmt.Sprintf("Subject: %s", msg.Subject),
		"MIME-Version: 1.0",
		`Content-Type: text/html; charset="UTF-8"`,
	}

	return strings.Join(headers, "\r\n") + "\r\n\r\n" + msg.HTMLBody
}
