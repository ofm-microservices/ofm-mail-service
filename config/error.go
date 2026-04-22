package config

import "fmt"

// WrapParseEnvConfigError annotates configuration parsing failures.
func WrapParseEnvConfigError(err error) error {
	return fmt.Errorf("parse env config: %w", err)
}
