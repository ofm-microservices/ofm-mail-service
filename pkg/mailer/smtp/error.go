package smtp

import (
	"errors"
	"fmt"
)

var (
	ErrEmptyHost        = errors.New("smtp host is empty")
	ErrInvalidPort      = errors.New("smtp port must be greater than zero")
	ErrEmptySenderEmail = errors.New("smtp sender email is empty")
	ErrEmptyPassword    = errors.New("smtp password is empty")
	ErrInvalidMode      = errors.New("smtp mode is invalid")
	ErrNilLogger        = errors.New("logger is nil")
)

// WrapDialSMTPError annotates SMTP dial failures.
func WrapDialSMTPError(err error) error {
	return fmt.Errorf("dial smtp: %w", err)
}

// WrapCreateSMTPClientError annotates SMTP client construction failures.
func WrapCreateSMTPClientError(err error) error {
	return fmt.Errorf("create smtp client: %w", err)
}

// WrapStartTLSError annotates StartTLS negotiation failures.
func WrapStartTLSError(err error) error {
	return fmt.Errorf("starttls smtp client: %w", err)
}

// WrapSMTPAuthError annotates SMTP authentication failures.
func WrapSMTPAuthError(err error) error {
	return fmt.Errorf("smtp auth: %w", err)
}

// WrapSMTPMailFromError annotates sender-envelope failures.
func WrapSMTPMailFromError(err error) error {
	return fmt.Errorf("smtp mail from: %w", err)
}

// WrapSMTPRcptToError annotates recipient-envelope failures.
func WrapSMTPRcptToError(err error) error {
	return fmt.Errorf("smtp rcpt to: %w", err)
}

// WrapSMTPDataError annotates SMTP DATA command failures.
func WrapSMTPDataError(err error) error {
	return fmt.Errorf("smtp data: %w", err)
}

// WrapSMTPWriteError annotates message body write failures.
func WrapSMTPWriteError(err error) error {
	return fmt.Errorf("smtp write message: %w", err)
}
