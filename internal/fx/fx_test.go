package appfx

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/ofm-microservices/ofm-common/pkg/logging"
	"mail-service/config"
	app "mail-service/internal/application"
	mail "mail-service/internal/domain"
	eventbroker "mail-service/internal/presentation/event_broker"
	events "mail-service/internal/presentation/event_broker/nats"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/fx"
)

func TestFX(t *testing.T) {
	t.Helper()
	RegisterFailHandler(Fail)
	RunSpecs(t, "FX Suite")
}

var (
	fxNATSContainer testcontainers.Container
	fxNATSURL       string
)

var _ = BeforeSuite(func() {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	container, url := startFXNATSContainer(ctx)
	fxNATSContainer = container
	fxNATSURL = url
})

var _ = AfterSuite(func() {
	if fxNATSContainer != nil {
		Expect(fxNATSContainer.Terminate(context.Background())).To(Succeed())
	}
})

type lifecycleStub struct {
	hooks []fx.Hook
}

func (l *lifecycleStub) Append(h fx.Hook) {
	l.hooks = append(l.hooks, h)
}

type brokerStub struct {
	runErr       error
	runCalls     int
	runCfg       config.PullConsumerConfig
	runHandler   any
	publishCalls int
}

func (b *brokerStub) Publish(context.Context, string, []byte) error { b.publishCalls++; return nil }
func (b *brokerStub) Subscribe(context.Context, string, eventbroker.MessageHandler) error {
	return nil
}
func (b *brokerStub) RunPullConsumer(_ context.Context, cfg config.PullConsumerConfig, handler eventbroker.MessageHandler) error {
	b.runCalls++
	b.runCfg = cfg
	b.runHandler = handler
	return b.runErr
}
func (b *brokerStub) Close() {}

type mailServiceStub struct{}

func (mailServiceStub) Send(context.Context, mail.SendRequest) (*mail.SendResult, error) {
	return &mail.SendResult{Status: "sent"}, nil
}

type subscriberStub struct {
	err        error
	calls      int
	lastCtx    context.Context
	cancelSeen bool
}

func (s *subscriberStub) Subscribe(ctx context.Context) error {
	s.calls++
	s.lastCtx = ctx
	select {
	case <-ctx.Done():
		s.cancelSeen = true
	default:
	}
	return s.err
}

var _ = Describe("FX providers", func() {
	var cfg *config.Config
	var lg logging.Logger

	BeforeEach(func() {
		var err error
		lg, err = logging.New("mail-service", "test", "debug")
		Expect(err).NotTo(HaveOccurred())
		cfg = &config.Config{
			App: config.AppConfig{Env: "test", LogLevel: "debug"},
			SMTP: config.SMTPConfig{
				Host:        "smtp.example.com",
				Port:        587,
				Mode:        "starttls",
				SenderEmail: "mailer@example.com",
				Password:    "secret",
				FromName:    "OFM",
			},
			NATS: config.NATSConfig{
				URL:                         "://bad-url",
				MailCommandsStream:          "MAIL_COMMANDS",
				MailEventsStream:            "MAIL_EVENTS",
				MailSendSubject:             "mail.send",
				MailSendResultSubject:       "mail.send.result",
				MailSendDurable:             "mail_service_send",
				MailBatchSize:               32,
				MailMaxWait:                 10 * time.Millisecond,
				MailWorkers:                 8,
				MailQueueSize:               500,
				MailAckWait:                 30 * time.Second,
				MailMaxDeliver:              5,
				MailAdaptiveCheckInterval:   2 * time.Second,
				MailAdaptiveMediumPending:   200,
				MailAdaptiveHighPending:     1000,
				MailAdaptiveLowBatchSize:    8,
				MailAdaptiveLowMaxWait:      25 * time.Millisecond,
				MailAdaptiveMediumBatchSize: 32,
				MailAdaptiveMediumMaxWait:   10 * time.Millisecond,
				MailAdaptiveHighBatchSize:   128,
				MailAdaptiveHighMaxWait:     2 * time.Millisecond,
			},
			Templates: config.TemplatesConfig{Dir: GinkgoT().TempDir()},
		}
	})

	It("loads config through ProvideConfig", func() {
		prevWD, err := os.Getwd()
		Expect(err).NotTo(HaveOccurred())
		defer func() {
			Expect(os.Chdir(prevWD)).To(Succeed())
			Expect(os.Unsetenv("NATS_URL")).To(Succeed())
			Expect(os.Unsetenv("SENDER_EMAIL")).To(Succeed())
			Expect(os.Unsetenv("EMAIL_PASSWORD")).To(Succeed())
		}()

		tmpDir := GinkgoT().TempDir()
		Expect(os.WriteFile(filepath.Join(tmpDir, ".env"), []byte("NATS_URL=nats://localhost:4222\nSENDER_EMAIL=mailer@example.com\nEMAIL_PASSWORD=secret\n"), 0o600)).To(Succeed())
		Expect(os.Chdir(tmpDir)).To(Succeed())

		loaded, err := ProvideConfig()

		Expect(err).NotTo(HaveOccurred())
		Expect(loaded.NATS.URL).To(Equal("nats://localhost:4222"))
		Expect(loaded.SMTP.SenderEmail).To(Equal("mailer@example.com"))
	})

	It("constructs a logger and registers a shutdown hook", func() {
		lc := &lifecycleStub{}

		logger, err := ProvideLogger(lc, cfg)

		Expect(err).NotTo(HaveOccurred())
		Expect(logger).NotTo(BeNil())
		Expect(lc.hooks).To(HaveLen(1))
		Expect(lc.hooks[0].OnStop).NotTo(BeNil())
		Expect(func() { _ = lc.hooks[0].OnStop(context.Background()) }).NotTo(Panic())
	})

	It("returns a logger construction error for an invalid log level", func() {
		lc := &lifecycleStub{}
		cfg.App.LogLevel = "bad-level"

		logger, err := ProvideLogger(lc, cfg)

		Expect(err).To(HaveOccurred())
		Expect(logger).To(BeNil())
		Expect(lc.hooks).To(BeEmpty())
	})

	It("returns an event broker construction error for an invalid nats url", func() {
		lc := &lifecycleStub{}

		broker, err := ProvideEventBroker(lc, cfg, lg)

		Expect(err).To(HaveOccurred())
		Expect(broker).To(BeNil())
		Expect(lc.hooks).To(BeEmpty())
	})

	It("returns a configured mail sender", func() {
		sender, err := ProvideMailSender(cfg, lg)

		Expect(err).NotTo(HaveOccurred())
		Expect(sender).NotTo(BeNil())
	})

	It("builds a template registry from grouped definitions", func() {
		Expect(os.WriteFile(filepath.Join(cfg.Templates.Dir, "email_code.html"), []byte("<html>{{.code}}</html>"), 0o600)).To(Succeed())
		params := templateRegistryParams{
			Cfg: cfg,
			Defs: []app.TemplateDefinition{{
				MessageType:      "email_code",
				SubjectTemplate:  "Code {{.code}}",
				HTMLTemplatePath: "/wrong/path/email_code.html",
			}},
		}

		registry, err := ProvideTemplateRegistry(params)

		Expect(err).NotTo(HaveOccurred())
		Expect(registry).NotTo(BeNil())
		email, err := registry.Render("email_code", map[string]any{"code": "123456"})
		Expect(err).NotTo(HaveOccurred())
		Expect(email.Subject).To(Equal("Code 123456"))
		Expect(email.HTMLBody).To(ContainSubstring("123456"))
	})

	It("wraps template registry build errors", func() {
		params := templateRegistryParams{
			Cfg: cfg,
			Defs: []app.TemplateDefinition{{
				MessageType:      "email_code",
				SubjectTemplate:  "Code",
				HTMLTemplatePath: filepath.Join(cfg.Templates.Dir, "missing.html"),
			}},
		}

		registry, err := ProvideTemplateRegistry(params)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("build template registry"))
		Expect(registry).To(BeNil())
	})

	It("constructs the application mail service", func() {
		Expect(os.WriteFile(filepath.Join(cfg.Templates.Dir, "welcome.html"), []byte("<html>{{.name}}</html>"), 0o600)).To(Succeed())
		registry, err := app.BuildTemplateRegistry([]app.TemplateDefinition{{
			MessageType:      "welcome",
			SubjectTemplate:  "Welcome {{.name}}",
			HTMLTemplatePath: filepath.Join(cfg.Templates.Dir, "welcome.html"),
		}})
		Expect(err).NotTo(HaveOccurred())

		service, err := ProvideMailService(mustProvideSender(cfg, lg), registry, lg)

		Expect(err).NotTo(HaveOccurred())
		Expect(service).NotTo(BeNil())
	})

	It("provides the expected template definitions", func() {
		Expect(ProvideEmailCodeTemplateDefinition().HTMLTemplatePath).To(Equal("email_code.html"))
		Expect(ProvideWelcomeTemplateDefinition().HTMLTemplatePath).To(Equal("welcome.html"))
		Expect(ProvidePasswordResetTemplateDefinition().HTMLTemplatePath).To(Equal("password_reset.html"))
	})

	It("constructs the mail command subscriber", func() {
		subscriber, err := ProvideMailCommandSubscriber(&brokerStub{}, mailServiceStub{}, cfg, events.NewDomainFailureReasonResolver(), lg)

		Expect(err).NotTo(HaveOccurred())
		Expect(subscriber).NotTo(BeNil())
	})

	It("logs startup through InvokeStartLog", func() {
		Expect(func() { InvokeStartLog(lg) }).NotTo(Panic())
	})

	It("returns bootstrap errors from InvokeEnsureStream", func() {
		err := InvokeEnsureStream(cfg, lg)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("connect to nats"))
	})

	It("ensures streams successfully against a real nats server", func() {
		cfg.NATS.URL = fxNATSURL
		cfg.NATS.MailCommandsStream = "MAIL_COMMANDS_FX"
		cfg.NATS.MailEventsStream = "MAIL_EVENTS_FX"
		cfg.NATS.MailSendSubject = "mail.send.fx"
		cfg.NATS.MailSendResultSubject = "mail.send.result.fx"

		err := InvokeEnsureStream(cfg, lg)

		Expect(err).NotTo(HaveOccurred())

		nc, err := nats.Connect(fxNATSURL)
		Expect(err).NotTo(HaveOccurred())
		defer nc.Close()

		js, err := nc.JetStream()
		Expect(err).NotTo(HaveOccurred())

		commandsInfo, err := js.StreamInfo(cfg.NATS.MailCommandsStream)
		Expect(err).NotTo(HaveOccurred())
		Expect(commandsInfo.Config.Subjects).To(ContainElement(cfg.NATS.MailSendSubject))

		eventsInfo, err := js.StreamInfo(cfg.NATS.MailEventsStream)
		Expect(err).NotTo(HaveOccurred())
		Expect(eventsInfo.Config.Subjects).To(ContainElement(cfg.NATS.MailSendResultSubject))
	})

	It("constructs an event broker and registers an on-stop hook against a real nats server", func() {
		cfg.NATS.URL = fxNATSURL
		lc := &lifecycleStub{}

		broker, err := ProvideEventBroker(lc, cfg, lg)

		Expect(err).NotTo(HaveOccurred())
		Expect(broker).NotTo(BeNil())
		Expect(lc.hooks).To(HaveLen(1))
		Expect(lc.hooks[0].OnStop).NotTo(BeNil())
		Expect(lc.hooks[0].OnStop(context.Background())).To(Succeed())
	})

	It("starts and stops the mail command subscriber through the lifecycle", func() {
		lc := &lifecycleStub{}
		sub := &subscriberStub{}

		InvokeSubscribeMailCommands(lc, sub, cfg, lg)

		Expect(lc.hooks).To(HaveLen(1))
		Expect(lc.hooks[0].OnStart).NotTo(BeNil())
		Expect(lc.hooks[0].OnStop).NotTo(BeNil())
		Expect(lc.hooks[0].OnStart(context.Background())).To(Succeed())
		Expect(sub.calls).To(Equal(1))
		Expect(sub.lastCtx).NotTo(BeNil())
		Expect(sub.lastCtx.Err()).NotTo(HaveOccurred())
		Expect(lc.hooks[0].OnStop(context.Background())).To(Succeed())
		Eventually(sub.lastCtx.Done()).Should(BeClosed())
	})

	It("returns subscriber startup errors from the lifecycle hook", func() {
		lc := &lifecycleStub{}
		sub := &subscriberStub{err: errors.New("subscribe failed")}

		InvokeSubscribeMailCommands(lc, sub, cfg, lg)

		Expect(lc.hooks[0].OnStart(context.Background())).To(MatchError("subscribe failed"))
	})
})

func mustProvideSender(cfg *config.Config, lg logging.Logger) app.MailSender {
	sender, err := ProvideMailSender(cfg, lg)
	Expect(err).NotTo(HaveOccurred())
	return sender
}

func startFXNATSContainer(ctx context.Context) (testcontainers.Container, string) {
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "nats:2.11-alpine",
			ExposedPorts: []string{"4222/tcp"},
			Cmd:          []string{"-js"},
			WaitingFor:   wait.ForListeningPort("4222/tcp"),
		},
		Started: true,
	})
	Expect(err).NotTo(HaveOccurred())

	host, err := container.Host(ctx)
	Expect(err).NotTo(HaveOccurred())
	port, err := container.MappedPort(ctx, "4222/tcp")
	Expect(err).NotTo(HaveOccurred())

	return container, "nats://" + host + ":" + port.Port()
}
