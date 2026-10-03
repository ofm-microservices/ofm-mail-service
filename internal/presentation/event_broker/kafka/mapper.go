package kafka

import (
	"encoding/json"
	"time"

	mail "mail-service/internal/domain"
)

type mailMapper struct{}

func newMailMapper() mailMessageMapper { return mailMapper{} }

func (mailMapper) ToSendRequest(cmd sendMailCommand) (mail.SendRequest, error) {
	return mail.SendRequest{SessionID: cmd.SessionID, ClientID: cmd.ClientID, UserID: cmd.UserID, RequestID: cmd.RequestID, CorrelationID: cmd.CorrelationID, MessageType: cmd.MessageType, To: cmd.To, Data: cmd.Data}, nil
}

func (mailMapper) ToFailureResultPayload(cmd sendMailCommand, reason string) ([]byte, error) {
	return json.Marshal(sendMailResult{SessionID: cmd.SessionID, ClientID: cmd.ClientID, UserID: cmd.UserID, RequestID: cmd.RequestID, CorrelationID: cmd.CorrelationID, MessageType: cmd.MessageType, To: cmd.To, Status: "failed", Error: reason, Timestamp: time.Now().UTC().Format(time.RFC3339Nano)})
}

func (mailMapper) ToSuccessResultPayload(result *mail.SendResult) ([]byte, error) {
	return json.Marshal(sendMailResult{SessionID: result.SessionID, ClientID: result.ClientID, UserID: result.UserID, RequestID: result.RequestID, CorrelationID: result.CorrelationID, MessageType: result.MessageType, To: result.To, Status: result.Status, Timestamp: time.Now().UTC().Format(time.RFC3339Nano)})
}
