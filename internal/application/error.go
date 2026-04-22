package service

import "errors"

var (
	ErrNilMailSender       = errors.New("mail sender is nil")
	ErrNilTemplateRegistry = errors.New("template registry is nil")
	ErrNilLogger           = errors.New("logger is nil")
)
