package kafka

import (
	"errors"

	app "mail-service/internal/application"
	mail "mail-service/internal/domain"
)

// FailureReasonResolver maps mail domain failures to stable event reasons.
type FailureReasonResolver interface {
	SendMailFailureReason(error) string
}

type domainFailureReasonResolver struct{}

// NewDomainFailureReasonResolver constructs the Kafka mail failure mapper.
func NewDomainFailureReasonResolver() FailureReasonResolver { return domainFailureReasonResolver{} }

func (domainFailureReasonResolver) SendMailFailureReason(err error) string {
	switch {
	case errors.Is(err, mail.ErrInvalidRecipientEmail):
		return "invalid recipient email"
	case errors.Is(err, mail.ErrInvalidMessageType):
		return "invalid message type"
	case errors.Is(err, mail.ErrTemplateNotFound):
		return "mail template not found"
	case errors.Is(err, mail.ErrTemplateSubjectEmpty):
		return "mail template subject is empty"
	case errors.Is(err, mail.ErrTemplateBodyEmpty):
		return "mail template body is empty"
	case errors.Is(err, mail.ErrFailedToRenderMail):
		return "failed to render mail template"
	case errors.Is(err, mail.ErrFailedToSendMail):
		return "failed to send mail"
	case app.IsDomainSendError(err):
		return "failed to process mail command"
	default:
		return "internal mail delivery error"
	}
}
