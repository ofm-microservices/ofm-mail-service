package nats

import (
	"encoding/json"
	mail "mail-service/internal/domain"
	"time"
)

type mailMessageMapper struct{}

func newMailMessageMapper() MailMessageMapper {
	return &mailMessageMapper{}
}

func (m *mailMessageMapper) ToSendRequest(cmd sendMailCommand) (mail.SendRequest, error) {
	data := make(map[string]any)
	if len(cmd.Data) > 0 {
		if err := json.Unmarshal(cmd.Data, &data); err != nil {
			return mail.SendRequest{}, WrapUnmarshalSendMailCommandError(err)
		}
	}

	return mail.SendRequest{
		SessionID:     cmd.SessionID,
		ClientID:      cmd.ClientID,
		UserID:        cmd.UserID,
		RequestID:     cmd.RequestID,
		CorrelationID: cmd.CorrelationID,
		MessageType:   cmd.MessageType,
		To:            cmd.To,
		Data:          data,
	}, nil
}

func (m *mailMessageMapper) ToFailureResultPayload(cmd sendMailCommand, reason string) ([]byte, error) {
	payload, err := json.Marshal(sendMailResult{
		SessionID:     cmd.SessionID,
		ClientID:      cmd.ClientID,
		UserID:        cmd.UserID,
		RequestID:     cmd.RequestID,
		CorrelationID: cmd.CorrelationID,
		MessageType:   cmd.MessageType,
		To:            cmd.To,
		Status:        "failed",
		Error:         reason,
		Timestamp:     time.Now().UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return nil, WrapMarshalSendMailResultError(err)
	}

	return payload, nil
}

func (m *mailMessageMapper) ToSuccessResultPayload(result *mail.SendResult) ([]byte, error) {
	payload, err := json.Marshal(sendMailResult{
		SessionID:     result.SessionID,
		ClientID:      result.ClientID,
		UserID:        result.UserID,
		RequestID:     result.RequestID,
		CorrelationID: result.CorrelationID,
		MessageType:   result.MessageType,
		To:            result.To,
		Status:        result.Status,
		Timestamp:     time.Now().UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return nil, WrapMarshalSendMailResultError(err)
	}

	return payload, nil
}
