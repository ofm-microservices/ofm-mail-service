package service

import (
	"context"
	mail "mail-service/internal/domain"
)

// MailService owns the rendering and delivery workflow for outbound email.
type MailService interface {
	// Send renders the template and delegates delivery to the configured sender.
	Send(ctx context.Context, req mail.SendRequest) (*mail.SendResult, error)
}

// MailSender defines the infrastructure adapter responsible for delivering a
// rendered email.
type MailSender interface {
	// Send transmits a rendered email through the concrete transport.
	Send(ctx context.Context, msg mail.Email) error
}

// TemplateRegistry resolves template definitions and renders them into final
// HTML output.
type TemplateRegistry interface {
	// Register adds one message-type definition to the registry.
	Register(def TemplateDefinition) error
	// Render resolves a message type into a concrete email.
	Render(messageType string, data map[string]any) (*mail.Email, error)
}

// TemplateDefinition binds one logical message type to its subject and HTML
// template.
type TemplateDefinition struct {
	MessageType      string
	SubjectTemplate  string
	HTMLTemplatePath string
}
