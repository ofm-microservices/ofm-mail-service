package service

import (
	"bytes"
	"errors"
	"fmt"
	htmpl "html/template"
	mail "mail-service/internal/domain"
	"os"
	"path/filepath"
	"strings"
	ttmpl "text/template"
)

type templateRegistry struct {
	defs map[string]compiledTemplate
}

type compiledTemplate struct {
	subject *ttmpl.Template
	body    *htmpl.Template
}

// NewTemplateRegistry constructs an empty template registry implementation.
func NewTemplateRegistry() TemplateRegistry {
	return &templateRegistry{
		defs: make(map[string]compiledTemplate),
	}
}

func (r *templateRegistry) Register(def TemplateDefinition) error {
	if strings.TrimSpace(def.MessageType) == "" {
		return mail.ErrInvalidMessageType
	}
	if strings.TrimSpace(def.SubjectTemplate) == "" {
		return mail.ErrTemplateSubjectEmpty
	}
	if strings.TrimSpace(def.HTMLTemplatePath) == "" {
		return mail.ErrTemplateBodyEmpty
	}

	subjectTpl, err := ttmpl.New(def.MessageType + "-subject").Parse(def.SubjectTemplate)
	if err != nil {
		return fmt.Errorf("%w: parse subject template: %v", mail.ErrFailedToRenderMail, err)
	}

	bodyBytes, err := os.ReadFile(filepath.Clean(def.HTMLTemplatePath))
	if err != nil {
		return fmt.Errorf("%w: read html template: %v", mail.ErrFailedToRenderMail, err)
	}

	bodyTpl, err := htmpl.New(filepath.Base(def.HTMLTemplatePath)).Parse(string(bodyBytes))
	if err != nil {
		return fmt.Errorf("%w: parse html template: %v", mail.ErrFailedToRenderMail, err)
	}

	r.defs[def.MessageType] = compiledTemplate{
		subject: subjectTpl,
		body:    bodyTpl,
	}
	return nil
}

func (r *templateRegistry) Render(messageType string, data map[string]any) (*mail.Email, error) {
	tpl, ok := r.defs[messageType]
	if !ok {
		return nil, mail.ErrTemplateNotFound
	}

	payload := data
	if payload == nil {
		payload = map[string]any{}
	}

	var subject bytes.Buffer
	if err := tpl.subject.Execute(&subject, payload); err != nil {
		return nil, fmt.Errorf("%w: execute subject template: %v", mail.ErrFailedToRenderMail, err)
	}

	var body bytes.Buffer
	if err := tpl.body.Execute(&body, payload); err != nil {
		return nil, fmt.Errorf("%w: execute html template: %v", mail.ErrFailedToRenderMail, err)
	}

	subjectText := strings.TrimSpace(subject.String())
	if subjectText == "" {
		return nil, mail.ErrTemplateSubjectEmpty
	}

	bodyText := strings.TrimSpace(body.String())
	if bodyText == "" {
		return nil, mail.ErrTemplateBodyEmpty
	}

	return &mail.Email{
		Subject:  subjectText,
		HTMLBody: bodyText,
	}, nil
}

// BuildTemplateRegistry registers all supplied template definitions in a fresh
// registry.
func BuildTemplateRegistry(defs []TemplateDefinition) (TemplateRegistry, error) {
	registry := NewTemplateRegistry()
	for _, def := range defs {
		if err := registry.Register(def); err != nil {
			return nil, err
		}
	}
	return registry, nil
}

// WrapTemplateDefinitionGroupError annotates grouped template-definition build
// failures.
func WrapTemplateDefinitionGroupError(err error) error {
	return fmt.Errorf("build template registry: %w", err)
}

// IsTemplateRegistryError reports whether the error belongs to the template
// registry/rendering failure set.
func IsTemplateRegistryError(err error) bool {
	return errors.Is(err, mail.ErrTemplateNotFound) ||
		errors.Is(err, mail.ErrTemplateSubjectEmpty) ||
		errors.Is(err, mail.ErrTemplateBodyEmpty) ||
		errors.Is(err, mail.ErrFailedToRenderMail)
}
