package kafka

import (
	"errors"
	"fmt"
)

var (
	ErrNilBroker                = errors.New("event broker is nil")
	ErrNilMailService           = errors.New("mail service is nil")
	ErrNilFailureReasonResolver = errors.New("failure reason resolver is nil")
	ErrNilLogger                = errors.New("logger is nil")
)

func wrapUnmarshal(err error) error { return fmt.Errorf("unmarshal send mail command: %w", err) }
