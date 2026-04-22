package nats

import (
	"errors"
	"fmt"
)

// ErrNilLogger is returned when bootstrap logging is requested without a
// logger.
var ErrNilLogger = errors.New("logger is nil")

// WrapConnectToNATSError annotates low-level NATS connection failures.
func WrapConnectToNATSError(err error) error {
	return fmt.Errorf("connect to nats: %w", err)
}

// WrapInitJetStreamContextError annotates JetStream initialization failures.
func WrapInitJetStreamContextError(err error) error {
	return fmt.Errorf("init jetstream context: %w", err)
}

// WrapEnsureStreamError annotates add-or-update stream bootstrap failures.
func WrapEnsureStreamError(streamName string, addErr, updateErr error) error {
	return fmt.Errorf("ensure stream %q: add err=%v, update err=%w", streamName, addErr, updateErr)
}
