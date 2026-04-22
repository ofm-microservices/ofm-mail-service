package mail

// Email is the fully rendered outbound message passed to the mail sender.
type Email struct {
	To       string
	Subject  string
	HTMLBody string
}

// SendRequest is the mail-service command payload used by orchestrators.
type SendRequest struct {
	SessionID     string
	ClientID      string
	UserID        string
	RequestID     string
	CorrelationID string
	MessageType   string
	To            string
	Data          map[string]any
}

// SendResult is the outcome published after a mail send attempt completes.
type SendResult struct {
	SessionID     string
	ClientID      string
	UserID        string
	RequestID     string
	CorrelationID string
	MessageType   string
	To            string
	Status        string
	Error         string
}
