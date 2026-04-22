package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ofm-microseervices/ofm-common/pkg/logging"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	mail "mail-service/internal/domain"
)

func TestApplication(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Application Suite")
}

var _ = Describe("MailService", func() {
	var (
		ctrl     *gomock.Controller
		sender   *MockMailSender
		registry *MockTemplateRegistry
		logger   logging.Logger
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		sender = NewMockMailSender(ctrl)
		registry = NewMockTemplateRegistry(ctrl)

		var err error
		logger, err = logging.New("mail-service", "test", "debug")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("New", func() {
		It("validates nil collaborators", func() {
			svc, err := New(nil, registry, logger)
			Expect(svc).To(BeNil())
			Expect(err).To(MatchError(ErrNilMailSender))

			svc, err = New(sender, nil, logger)
			Expect(svc).To(BeNil())
			Expect(err).To(MatchError(ErrNilTemplateRegistry))

			svc, err = New(sender, registry, nil)
			Expect(svc).To(BeNil())
			Expect(err).To(MatchError(ErrNilLogger))
		})
	})

	Describe("Send", func() {
		It("rejects an empty message type", func() {
			svc, err := New(sender, registry, logger)
			Expect(err).NotTo(HaveOccurred())

			result, err := svc.Send(context.Background(), mail.SendRequest{
				To: "user@example.com",
			})

			Expect(result).To(BeNil())
			Expect(err).To(MatchError(mail.ErrInvalidMessageType))
		})

		It("rejects an invalid recipient email", func() {
			svc, err := New(sender, registry, logger)
			Expect(err).NotTo(HaveOccurred())

			result, err := svc.Send(context.Background(), mail.SendRequest{
				MessageType: "welcome",
				To:          "not-an-email",
			})

			Expect(result).To(BeNil())
			Expect(err).To(MatchError(mail.ErrInvalidRecipientEmail))
		})

		It("returns template rendering errors", func() {
			svc, err := New(sender, registry, logger)
			Expect(err).NotTo(HaveOccurred())

			registry.EXPECT().
				Render("welcome", gomock.Any()).
				Return(nil, mail.ErrTemplateNotFound)

			result, err := svc.Send(context.Background(), mail.SendRequest{
				MessageType: "welcome",
				To:          "user@example.com",
			})

			Expect(result).To(BeNil())
			Expect(err).To(MatchError(mail.ErrTemplateNotFound))
		})

		It("returns sender errors after setting the recipient", func() {
			svc, err := New(sender, registry, logger)
			Expect(err).NotTo(HaveOccurred())

			registry.EXPECT().
				Render("welcome", map[string]any{"name": "Alex"}).
				Return(&mail.Email{Subject: "Welcome", HTMLBody: "<p>Hello</p>"}, nil)

			sender.EXPECT().
				Send(gomock.Any(), gomock.AssignableToTypeOf(mail.Email{})).
				DoAndReturn(func(_ context.Context, msg mail.Email) error {
					Expect(msg.To).To(Equal("user@example.com"))
					Expect(msg.Subject).To(Equal("Welcome"))
					Expect(msg.HTMLBody).To(Equal("<p>Hello</p>"))
					return mail.ErrFailedToSendMail
				})

			result, err := svc.Send(context.Background(), mail.SendRequest{
				MessageType: "welcome",
				To:          "user@example.com",
				Data:        map[string]any{"name": "Alex"},
			})

			Expect(result).To(BeNil())
			Expect(err).To(MatchError(mail.ErrFailedToSendMail))
		})

		It("returns a success result when rendering and sending succeed", func() {
			svc, err := New(sender, registry, logger)
			Expect(err).NotTo(HaveOccurred())

			registry.EXPECT().
				Render("welcome", map[string]any{"name": "Alex"}).
				Return(&mail.Email{Subject: "Welcome", HTMLBody: "<p>Hello</p>"}, nil)

			sender.EXPECT().
				Send(gomock.Any(), gomock.AssignableToTypeOf(mail.Email{})).
				DoAndReturn(func(_ context.Context, msg mail.Email) error {
					Expect(msg.To).To(Equal("user@example.com"))
					return nil
				})

			result, err := svc.Send(context.Background(), mail.SendRequest{
				SessionID:     "session-1",
				ClientID:      "client-1",
				UserID:        "user-1",
				RequestID:     "request-1",
				CorrelationID: "corr-1",
				MessageType:   "welcome",
				To:            "user@example.com",
				Data:          map[string]any{"name": "Alex"},
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())
			Expect(result).To(Equal(&mail.SendResult{
				SessionID:     "session-1",
				ClientID:      "client-1",
				UserID:        "user-1",
				RequestID:     "request-1",
				CorrelationID: "corr-1",
				MessageType:   "welcome",
				To:            "user@example.com",
				Status:        "success",
			}))
		})
	})
})

var _ = Describe("TemplateRegistry", func() {
	Describe("Register", func() {
		It("validates the template definition", func() {
			registry := NewTemplateRegistry()

			Expect(registry.Register(TemplateDefinition{})).To(MatchError(mail.ErrInvalidMessageType))
			Expect(registry.Register(TemplateDefinition{MessageType: "welcome"})).To(MatchError(mail.ErrTemplateSubjectEmpty))
			Expect(registry.Register(TemplateDefinition{
				MessageType:     "welcome",
				SubjectTemplate: "Welcome",
			})).To(MatchError(mail.ErrTemplateBodyEmpty))
		})

		It("wraps file read failures", func() {
			registry := NewTemplateRegistry()

			err := registry.Register(TemplateDefinition{
				MessageType:      "welcome",
				SubjectTemplate:  "Welcome {{.name}}",
				HTMLTemplatePath: "/definitely/missing/template.html",
			})

			Expect(err).To(MatchError(ContainSubstring("read html template")))
			Expect(err).To(MatchError(ContainSubstring(mail.ErrFailedToRenderMail.Error())))
		})

		It("wraps invalid subject templates", func() {
			registry := NewTemplateRegistry()
			templatePath := writeTemplateFile("<p>Hello</p>")

			err := registry.Register(TemplateDefinition{
				MessageType:      "welcome",
				SubjectTemplate:  "{{",
				HTMLTemplatePath: templatePath,
			})

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("parse subject template"))
		})

		It("wraps invalid html templates", func() {
			registry := NewTemplateRegistry()
			templatePath := writeTemplateFile("{{")

			err := registry.Register(TemplateDefinition{
				MessageType:      "welcome",
				SubjectTemplate:  "Welcome",
				HTMLTemplatePath: templatePath,
			})

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("parse html template"))
		})
	})

	Describe("Render", func() {
		It("renders a registered template", func() {
			registry := NewTemplateRegistry()
			templatePath := writeTemplateFile("<p>Hello {{.name}}</p>")

			Expect(registry.Register(TemplateDefinition{
				MessageType:      "welcome",
				SubjectTemplate:  "Welcome {{.name}}",
				HTMLTemplatePath: templatePath,
			})).To(Succeed())

			email, err := registry.Render("welcome", map[string]any{"name": "Alex"})

			Expect(err).NotTo(HaveOccurred())
			Expect(email).To(Equal(&mail.Email{
				Subject:  "Welcome Alex",
				HTMLBody: "<p>Hello Alex</p>",
			}))
		})

		It("uses an empty payload map when render data is nil", func() {
			registry := NewTemplateRegistry()
			templatePath := writeTemplateFile("<p>Hello</p>")

			Expect(registry.Register(TemplateDefinition{
				MessageType:      "welcome",
				SubjectTemplate:  "Welcome",
				HTMLTemplatePath: templatePath,
			})).To(Succeed())

			email, err := registry.Render("welcome", nil)

			Expect(err).NotTo(HaveOccurred())
			Expect(email).To(Equal(&mail.Email{
				Subject:  "Welcome",
				HTMLBody: "<p>Hello</p>",
			}))
		})

		It("returns template not found when the type is unknown", func() {
			registry := NewTemplateRegistry()

			email, err := registry.Render("missing", nil)

			Expect(email).To(BeNil())
			Expect(err).To(MatchError(mail.ErrTemplateNotFound))
		})

		It("returns subject empty when the rendered subject is blank", func() {
			registry := NewTemplateRegistry()
			templatePath := writeTemplateFile("<p>Hello</p>")

			Expect(registry.Register(TemplateDefinition{
				MessageType:      "welcome",
				SubjectTemplate:  "{{if .missing}}value{{end}}",
				HTMLTemplatePath: templatePath,
			})).To(Succeed())

			email, err := registry.Render("welcome", map[string]any{})

			Expect(email).To(BeNil())
			Expect(err).To(MatchError(mail.ErrTemplateSubjectEmpty))
		})

		It("returns body empty when the rendered body is blank", func() {
			registry := NewTemplateRegistry()
			templatePath := writeTemplateFile("{{.missing}}")

			Expect(registry.Register(TemplateDefinition{
				MessageType:      "welcome",
				SubjectTemplate:  "Welcome",
				HTMLTemplatePath: templatePath,
			})).To(Succeed())

			email, err := registry.Render("welcome", map[string]any{})

			Expect(email).To(BeNil())
			Expect(err).To(MatchError(mail.ErrTemplateBodyEmpty))
		})

		It("wraps subject template execution failures", func() {
			registry := NewTemplateRegistry()
			templatePath := writeTemplateFile("<p>Hello</p>")

			Expect(registry.Register(TemplateDefinition{
				MessageType:      "welcome",
				SubjectTemplate:  "{{index . 1}}",
				HTMLTemplatePath: templatePath,
			})).To(Succeed())

			email, err := registry.Render("welcome", map[string]any{"name": "Alex"})

			Expect(email).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("execute subject template"))
		})

		It("wraps html template execution failures", func() {
			registry := NewTemplateRegistry()
			templatePath := writeTemplateFile("{{index . 1}}")

			Expect(registry.Register(TemplateDefinition{
				MessageType:      "welcome",
				SubjectTemplate:  "Welcome",
				HTMLTemplatePath: templatePath,
			})).To(Succeed())

			email, err := registry.Render("welcome", map[string]any{"name": "Alex"})

			Expect(email).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("execute html template"))
		})
	})

	Describe("BuildTemplateRegistry", func() {
		It("builds a registry from definitions", func() {
			templatePath := writeTemplateFile("<p>Hello {{.name}}</p>")

			registry, err := BuildTemplateRegistry([]TemplateDefinition{
				{
					MessageType:      "welcome",
					SubjectTemplate:  "Welcome {{.name}}",
					HTMLTemplatePath: templatePath,
				},
			})

			Expect(err).NotTo(HaveOccurred())
			email, err := registry.Render("welcome", map[string]any{"name": "Alex"})
			Expect(err).NotTo(HaveOccurred())
			Expect(email.Subject).To(Equal("Welcome Alex"))
		})

		It("returns the first registration error", func() {
			registry, err := BuildTemplateRegistry([]TemplateDefinition{{}})

			Expect(registry).To(BeNil())
			Expect(err).To(MatchError(mail.ErrInvalidMessageType))
		})
	})

	Describe("Helpers", func() {
		It("wraps grouped definition errors", func() {
			err := WrapTemplateDefinitionGroupError(errors.New("boom"))

			Expect(err).To(MatchError(ContainSubstring("build template registry")))
			Expect(err).To(MatchError(ContainSubstring("boom")))
		})

		It("classifies template registry errors", func() {
			Expect(IsTemplateRegistryError(mail.ErrTemplateNotFound)).To(BeTrue())
			Expect(IsTemplateRegistryError(mail.ErrTemplateSubjectEmpty)).To(BeTrue())
			Expect(IsTemplateRegistryError(mail.ErrTemplateBodyEmpty)).To(BeTrue())
			Expect(IsTemplateRegistryError(mail.ErrFailedToRenderMail)).To(BeTrue())
			Expect(IsTemplateRegistryError(errors.New("boom"))).To(BeFalse())
		})

		It("classifies domain send errors", func() {
			Expect(IsDomainSendError(mail.ErrInvalidRecipientEmail)).To(BeTrue())
			Expect(IsDomainSendError(mail.ErrInvalidMessageType)).To(BeTrue())
			Expect(IsDomainSendError(mail.ErrTemplateNotFound)).To(BeTrue())
			Expect(IsDomainSendError(mail.ErrTemplateSubjectEmpty)).To(BeTrue())
			Expect(IsDomainSendError(mail.ErrTemplateBodyEmpty)).To(BeTrue())
			Expect(IsDomainSendError(mail.ErrFailedToRenderMail)).To(BeTrue())
			Expect(IsDomainSendError(mail.ErrFailedToSendMail)).To(BeTrue())
			Expect(IsDomainSendError(errors.New("boom"))).To(BeFalse())
		})
	})
})

func writeTemplateFile(body string) string {
	path := filepath.Join(GinkgoT().TempDir(), "template.html")
	Expect(os.WriteFile(path, []byte(body), 0o600)).To(Succeed())
	return path
}
