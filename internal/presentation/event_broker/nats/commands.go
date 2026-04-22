package nats

import "encoding/json"

type sendMailCommand struct {
	SessionID     string          `json:"session_id,omitempty"`
	ClientID      string          `json:"client_id,omitempty"`
	UserID        string          `json:"user_id,omitempty"`
	RequestID     string          `json:"request_id,omitempty"`
	CorrelationID string          `json:"correlation_id,omitempty"`
	MessageType   string          `json:"message_type"`
	To            string          `json:"to"`
	Data          json.RawMessage `json:"data,omitempty"`
}

type sendMailResult struct {
	SessionID     string `json:"session_id,omitempty"`
	ClientID      string `json:"client_id,omitempty"`
	UserID        string `json:"user_id,omitempty"`
	RequestID     string `json:"request_id,omitempty"`
	CorrelationID string `json:"correlation_id,omitempty"`
	MessageType   string `json:"message_type"`
	To            string `json:"to"`
	Status        string `json:"status"`
	Error         string `json:"error,omitempty"`
	Timestamp     string `json:"timestamp"`
}
