package service

import (
	"context"
	"errors"
	"github.com/ofm-microseervices/ofm-common/pkg/logging"
	mail "mail-service/internal/domain"
	netmail "net/mail"
	"strings"
)

type mailService struct {
	sender   MailSender
	registry TemplateRegistry
	log      logging.Logger
}

// New constructs the mail application service.
func New(sender MailSender, registry TemplateRegistry, log logging.Logger) (MailService, error) {
	if sender == nil {
		return nil, ErrNilMailSender
	}
	if registry == nil {
		return nil, ErrNilTemplateRegistry
	}
	if log == nil {
		return nil, ErrNilLogger
	}

	return &mailService{
		sender:   sender,
		registry: registry,
		log:      log.With(logging.String("module", "application")),
	}, nil
}

// Send validates the request, renders the template, and sends the email.
func (s *mailService) Send(ctx context.Context, req mail.SendRequest) (*mail.SendResult, error) {
	s.log.Info("send mail command received",
		logging.String("message_type", req.MessageType),
		logging.String("to", req.To),
		logging.String("request_id", req.RequestID),
		logging.String("correlation_id", req.CorrelationID),
	)

	if strings.TrimSpace(req.MessageType) == "" {
		return nil, mail.ErrInvalidMessageType
	}
	if _, err := netmail.ParseAddress(req.To); err != nil {
		return nil, mail.ErrInvalidRecipientEmail
	}

	email, err := s.registry.Render(req.MessageType, req.Data)
	if err != nil {
		s.log.Error("render mail template failed",
			logging.String("message_type", req.MessageType),
			logging.String("to", req.To),
			logging.Err(err),
		)
		return nil, err
	}
	email.To = req.To

	if err := s.sender.Send(ctx, *email); err != nil {
		s.log.Error("send mail failed",
			logging.String("message_type", req.MessageType),
			logging.String("to", req.To),
			logging.Err(err),
		)
		return nil, err
	}

	s.log.Info("mail sent",
		logging.String("message_type", req.MessageType),
		logging.String("to", req.To),
	)

	return &mail.SendResult{
		SessionID:     req.SessionID,
		ClientID:      req.ClientID,
		UserID:        req.UserID,
		RequestID:     req.RequestID,
		CorrelationID: req.CorrelationID,
		MessageType:   req.MessageType,
		To:            req.To,
		Status:        "success",
	}, nil
}

// IsDomainSendError reports whether err is one of the expected domain-level
// mail validation or rendering failures.
func IsDomainSendError(err error) bool {
	return errors.Is(err, mail.ErrInvalidRecipientEmail) ||
		errors.Is(err, mail.ErrInvalidMessageType) ||
		errors.Is(err, mail.ErrTemplateNotFound) ||
		errors.Is(err, mail.ErrTemplateSubjectEmpty) ||
		errors.Is(err, mail.ErrTemplateBodyEmpty) ||
		errors.Is(err, mail.ErrFailedToRenderMail) ||
		errors.Is(err, mail.ErrFailedToSendMail)
}
