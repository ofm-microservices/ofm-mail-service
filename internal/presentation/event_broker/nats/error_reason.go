package nats

import (
	"errors"
	app "mail-service/internal/application"
	mail "mail-service/internal/domain"
)

// FailureReasonResolver maps domain errors to stable saga-facing failure
// reasons.
type FailureReasonResolver interface {
	SendMailFailureReason(err error) string
}

// DomainFailureReasonResolver is the default domain-error-to-string mapper for
// mail delivery results.
type DomainFailureReasonResolver struct{}

// NewDomainFailureReasonResolver constructs the default failure reason
// resolver.
func NewDomainFailureReasonResolver() FailureReasonResolver {
	return &DomainFailureReasonResolver{}
}

func (r *DomainFailureReasonResolver) SendMailFailureReason(err error) string {
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
