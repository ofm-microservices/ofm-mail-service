package kafka

import (
	"context"

	mail "mail-service/internal/domain"
)

// MailCommandSubscriber consumes mail commands from Kafka.
type MailCommandSubscriber interface {
	Subscribe(context.Context) error
}

type mailMessageMapper interface {
	ToSendRequest(sendMailCommand) (mail.SendRequest, error)
	ToFailureResultPayload(sendMailCommand, string) ([]byte, error)
	ToSuccessResultPayload(*mail.SendResult) ([]byte, error)
}

type failureReasonResolver interface {
	SendMailFailureReason(error) string
}
