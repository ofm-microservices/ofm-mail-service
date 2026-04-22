package mail

import "errors"

var (
	ErrInvalidRecipientEmail = errors.New("invalid recipient email")
	ErrInvalidMessageType    = errors.New("invalid message type")
	ErrTemplateNotFound      = errors.New("mail template not found")
	ErrTemplateSubjectEmpty  = errors.New("mail template subject is empty")
	ErrTemplateBodyEmpty     = errors.New("mail template body is empty")
	ErrFailedToRenderMail    = errors.New("failed to render mail")
	ErrFailedToSendMail      = errors.New("failed to send mail")
)
