package service

import (
	"context"
	"reflect"

	"go.uber.org/mock/gomock"
	mail "mail-service/internal/domain"
)

type MockMailSender struct {
	ctrl     *gomock.Controller
	recorder *MockMailSenderMockRecorder
}

type MockMailSenderMockRecorder struct {
	mock *MockMailSender
}

func NewMockMailSender(ctrl *gomock.Controller) *MockMailSender {
	mock := &MockMailSender{ctrl: ctrl}
	mock.recorder = &MockMailSenderMockRecorder{mock}
	return mock
}

func (m *MockMailSender) EXPECT() *MockMailSenderMockRecorder {
	return m.recorder
}

func (m *MockMailSender) Send(ctx context.Context, msg mail.Email) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Send", ctx, msg)
	ret0, _ := ret[0].(error)
	return ret0
}

func (mr *MockMailSenderMockRecorder) Send(ctx, msg any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Send", reflect.TypeOf((*MockMailSender)(nil).Send), ctx, msg)
}

type MockTemplateRegistry struct {
	ctrl     *gomock.Controller
	recorder *MockTemplateRegistryMockRecorder
}

type MockTemplateRegistryMockRecorder struct {
	mock *MockTemplateRegistry
}

func NewMockTemplateRegistry(ctrl *gomock.Controller) *MockTemplateRegistry {
	mock := &MockTemplateRegistry{ctrl: ctrl}
	mock.recorder = &MockTemplateRegistryMockRecorder{mock}
	return mock
}

func (m *MockTemplateRegistry) EXPECT() *MockTemplateRegistryMockRecorder {
	return m.recorder
}

func (m *MockTemplateRegistry) Register(def TemplateDefinition) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Register", def)
	ret0, _ := ret[0].(error)
	return ret0
}

func (mr *MockTemplateRegistryMockRecorder) Register(def any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Register", reflect.TypeOf((*MockTemplateRegistry)(nil).Register), def)
}

func (m *MockTemplateRegistry) Render(messageType string, data any) (*mail.Email, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Render", messageType, data)
	ret0, _ := ret[0].(*mail.Email)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (mr *MockTemplateRegistryMockRecorder) Render(messageType, data any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Render", reflect.TypeOf((*MockTemplateRegistry)(nil).Render), messageType, data)
}
