package nats

type sendMailCommand struct {
	SessionID     string `json:"session_id,omitempty"`
	ClientID      string `json:"client_id,omitempty"`
	UserID        string `json:"user_id,omitempty"`
	RequestID     string `json:"request_id,omitempty"`
	CorrelationID string `json:"correlation_id,omitempty"`
	MessageType   string `json:"message_type"`
	To            string `json:"to"`
	Data          any    `json:"data"`
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
