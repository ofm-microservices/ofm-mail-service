package nats

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/ofm-microservices/ofm-common/pkg/logging"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	"mail-service/config"
	mail "mail-service/internal/domain"
	eventbroker "mail-service/internal/presentation/event_broker"
)

type stubMailMessageMapper struct {
	sendReq    mail.SendRequest
	sendErr    error
	failureErr error
	successErr error
}

func (m *stubMailMessageMapper) ToSendRequest(sendMailCommand) (mail.SendRequest, error) {
	return m.sendReq, m.sendErr
}

func (m *stubMailMessageMapper) ToFailureResultPayload(sendMailCommand, string) ([]byte, error) {
	return nil, m.failureErr
}

func (m *stubMailMessageMapper) ToSuccessResultPayload(*mail.SendResult) ([]byte, error) {
	return nil, m.successErr
}

type stubPullConsumerRuntime struct {
	started bool
}

func (r *stubPullConsumerRuntime) Start(context.Context) {
	r.started = true
}

type stubPullConsumerRuntimeFactory struct {
	runtime PullConsumerRuntime
	err     error
}

func (f *stubPullConsumerRuntimeFactory) Create(*nats.Conn, logging.Logger, config.PullConsumerConfig, eventbroker.MessageHandler) (PullConsumerRuntime, error) {
	return f.runtime, f.err
}

func TestNATS(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "NATS Suite")
}

var _ = Describe("MailCommandSubscriber", func() {
	var (
		ctrl    *gomock.Controller
		broker  *MockEventBroker
		service *MockMailService
		logger  logging.Logger
		cfg     config.NATSConfig
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		broker = NewMockEventBroker(ctrl)
		service = NewMockMailService(ctrl)

		var err error
		logger, err = logging.New("mail-service", "test", "debug")
		Expect(err).NotTo(HaveOccurred())

		cfg = config.NATSConfig{
			MailCommandsStream:          "MAIL_COMMANDS",
			MailSendSubject:             "mail.send",
			MailSendResultSubject:       "mail.send.result",
			MailSendDurable:             "mail_service_send",
			MailBatchSize:               10,
			MailMaxWait:                 5 * time.Millisecond,
			MailWorkers:                 2,
			MailQueueSize:               10,
			MailAckWait:                 2 * time.Second,
			MailMaxDeliver:              3,
			MailAdaptiveEnabled:         true,
			MailAdaptiveCheckInterval:   2 * time.Second,
			MailAdaptiveMediumPending:   10,
			MailAdaptiveHighPending:     20,
			MailAdaptiveLowBatchSize:    1,
			MailAdaptiveLowMaxWait:      50 * time.Millisecond,
			MailAdaptiveMediumBatchSize: 5,
			MailAdaptiveMediumMaxWait:   10 * time.Millisecond,
			MailAdaptiveHighBatchSize:   10,
			MailAdaptiveHighMaxWait:     2 * time.Millisecond,
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("NewMailCommandSubscriber", func() {
		It("validates nil collaborators", func() {
			sub, err := NewMailCommandSubscriber(nil, service, cfg, NewDomainFailureReasonResolver(), logger)
			Expect(sub).To(BeNil())
			Expect(err).To(MatchError(ErrNilBroker))

			sub, err = NewMailCommandSubscriber(broker, nil, cfg, NewDomainFailureReasonResolver(), logger)
			Expect(sub).To(BeNil())
			Expect(err).To(MatchError(ErrNilMailService))

			sub, err = NewMailCommandSubscriber(broker, service, cfg, nil, logger)
			Expect(sub).To(BeNil())
			Expect(err).To(MatchError(ErrNilFailureReasonResolver))

			sub, err = NewMailCommandSubscriber(broker, service, cfg, NewDomainFailureReasonResolver(), nil)
			Expect(sub).To(BeNil())
			Expect(err).To(MatchError(ErrNilLogger))
		})
	})

	Describe("Subscribe", func() {
		It("returns the pull-consumer registration error", func() {
			sub, err := NewMailCommandSubscriber(broker, service, cfg, NewDomainFailureReasonResolver(), logger)
			Expect(err).NotTo(HaveOccurred())

			broker.EXPECT().
				RunPullConsumer(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(errors.New("boom"))

			err = sub.Subscribe(context.Background())

			Expect(err).To(MatchError("boom"))
		})

		It("publishes a failure result when mail delivery fails", func() {
			sub, err := NewMailCommandSubscriber(broker, service, cfg, NewDomainFailureReasonResolver(), logger)
			Expect(err).NotTo(HaveOccurred())

			var handler eventbroker.MessageHandler
			broker.EXPECT().
				RunPullConsumer(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, consumerCfg config.PullConsumerConfig, h eventbroker.MessageHandler) error {
					handler = h
					Expect(consumerCfg.Subject).To(Equal(cfg.MailSendSubject))
					Expect(consumerCfg.Stream).To(Equal(cfg.MailCommandsStream))
					return nil
				})

			Expect(sub.Subscribe(context.Background())).To(Succeed())

			service.EXPECT().
				Send(gomock.Any(), gomock.AssignableToTypeOf(mail.SendRequest{})).
				Return(nil, mail.ErrFailedToSendMail)

			broker.EXPECT().
				Publish(gomock.Any(), cfg.MailSendResultSubject, gomock.Any()).
				DoAndReturn(func(_ context.Context, _ string, payload []byte) error {
					var result sendMailResult
					Expect(json.Unmarshal(payload, &result)).To(Succeed())
					Expect(result.Status).To(Equal("failed"))
					Expect(result.Error).To(Equal("failed to send mail"))
					Expect(result.MessageType).To(Equal("email_code"))
					return nil
				})

			payload, err := json.Marshal(sendMailCommand{
				SessionID:   "session-1",
				ClientID:    "client-1",
				UserID:      "user-1",
				RequestID:   "request-1",
				MessageType: "email_code",
				To:          "user@example.com",
				Data:        json.RawMessage(`{"name":"Alex"}`),
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(handler(context.Background(), cfg.MailSendSubject, payload)).To(Succeed())
		})

		It("publishes a success result when mail delivery succeeds", func() {
			sub, err := NewMailCommandSubscriber(broker, service, cfg, NewDomainFailureReasonResolver(), logger)
			Expect(err).NotTo(HaveOccurred())

			var handler eventbroker.MessageHandler
			broker.EXPECT().
				RunPullConsumer(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, _ config.PullConsumerConfig, h eventbroker.MessageHandler) error {
					handler = h
					return nil
				})

			Expect(sub.Subscribe(context.Background())).To(Succeed())

			service.EXPECT().
				Send(gomock.Any(), gomock.AssignableToTypeOf(mail.SendRequest{})).
				Return(&mail.SendResult{
					SessionID:     "session-1",
					ClientID:      "client-1",
					UserID:        "user-1",
					RequestID:     "request-1",
					CorrelationID: "corr-1",
					MessageType:   "email_code",
					To:            "user@example.com",
					Status:        "success",
				}, nil)

			broker.EXPECT().
				Publish(gomock.Any(), cfg.MailSendResultSubject, gomock.Any()).
				DoAndReturn(func(_ context.Context, _ string, payload []byte) error {
					var result sendMailResult
					Expect(json.Unmarshal(payload, &result)).To(Succeed())
					Expect(result.Status).To(Equal("success"))
					Expect(result.To).To(Equal("user@example.com"))
					return nil
				})

			payload, err := json.Marshal(sendMailCommand{
				SessionID:     "session-1",
				ClientID:      "client-1",
				UserID:        "user-1",
				RequestID:     "request-1",
				CorrelationID: "corr-1",
				MessageType:   "email_code",
				To:            "user@example.com",
				Data:          json.RawMessage(`{"name":"Alex"}`),
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(handler(context.Background(), cfg.MailSendSubject, payload)).To(Succeed())
		})

		It("returns an unmarshal error for invalid command payloads", func() {
			sub, err := NewMailCommandSubscriber(broker, service, cfg, NewDomainFailureReasonResolver(), logger)
			Expect(err).NotTo(HaveOccurred())

			var handler eventbroker.MessageHandler
			broker.EXPECT().
				RunPullConsumer(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, _ config.PullConsumerConfig, h eventbroker.MessageHandler) error {
					handler = h
					return nil
				})

			Expect(sub.Subscribe(context.Background())).To(Succeed())
			Expect(handler(context.Background(), cfg.MailSendSubject, []byte("{"))).
				To(MatchError(ContainSubstring("unmarshal send mail command")))
		})

		It("returns request mapping errors", func() {
			sub, err := NewMailCommandSubscriber(broker, service, cfg, NewDomainFailureReasonResolver(), logger)
			Expect(err).NotTo(HaveOccurred())
			sub.(*mailCommandSubscriber).mapr = &stubMailMessageMapper{sendErr: errors.New("map request failed")}

			var handler eventbroker.MessageHandler
			broker.EXPECT().RunPullConsumer(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, _ config.PullConsumerConfig, h eventbroker.MessageHandler) error {
					handler = h
					return nil
				})

			Expect(sub.Subscribe(context.Background())).To(Succeed())
			payload, err := json.Marshal(sendMailCommand{MessageType: "email_code", To: "user@example.com"})
			Expect(err).NotTo(HaveOccurred())
			Expect(handler(context.Background(), cfg.MailSendSubject, payload)).To(MatchError("map request failed"))
		})

		It("returns failure-result mapping errors", func() {
			sub, err := NewMailCommandSubscriber(broker, service, cfg, NewDomainFailureReasonResolver(), logger)
			Expect(err).NotTo(HaveOccurred())
			sub.(*mailCommandSubscriber).mapr = &stubMailMessageMapper{
				sendReq:    mail.SendRequest{MessageType: "email_code", To: "user@example.com"},
				failureErr: errors.New("map failure result failed"),
			}

			var handler eventbroker.MessageHandler
			broker.EXPECT().RunPullConsumer(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, _ config.PullConsumerConfig, h eventbroker.MessageHandler) error {
					handler = h
					return nil
				})

			Expect(sub.Subscribe(context.Background())).To(Succeed())
			service.EXPECT().Send(gomock.Any(), gomock.Any()).Return(nil, mail.ErrFailedToSendMail)
			payload, err := json.Marshal(sendMailCommand{MessageType: "email_code", To: "user@example.com"})
			Expect(err).NotTo(HaveOccurred())
			Expect(handler(context.Background(), cfg.MailSendSubject, payload)).To(MatchError("map failure result failed"))
		})

		It("returns success-result mapping errors", func() {
			sub, err := NewMailCommandSubscriber(broker, service, cfg, NewDomainFailureReasonResolver(), logger)
			Expect(err).NotTo(HaveOccurred())
			sub.(*mailCommandSubscriber).mapr = &stubMailMessageMapper{
				sendReq:    mail.SendRequest{MessageType: "email_code", To: "user@example.com"},
				successErr: errors.New("map success result failed"),
			}

			var handler eventbroker.MessageHandler
			broker.EXPECT().RunPullConsumer(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, _ config.PullConsumerConfig, h eventbroker.MessageHandler) error {
					handler = h
					return nil
				})

			Expect(sub.Subscribe(context.Background())).To(Succeed())
			service.EXPECT().Send(gomock.Any(), gomock.Any()).Return(&mail.SendResult{Status: "success"}, nil)
			payload, err := json.Marshal(sendMailCommand{MessageType: "email_code", To: "user@example.com"})
			Expect(err).NotTo(HaveOccurred())
			Expect(handler(context.Background(), cfg.MailSendSubject, payload)).To(MatchError("map success result failed"))
		})
	})
})

var _ = Describe("MailMessageMapper", func() {
	Describe("ToSendRequest", func() {
		It("maps a command payload into an application request", func() {
			mapr := newMailMessageMapper()

			req, err := mapr.ToSendRequest(sendMailCommand{
				SessionID:     "session-1",
				ClientID:      "client-1",
				UserID:        "user-1",
				RequestID:     "request-1",
				CorrelationID: "corr-1",
				MessageType:   "email_code",
				To:            "user@example.com",
				Data:          json.RawMessage(`{"name":"Alex"}`),
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(req).To(Equal(mail.SendRequest{
				SessionID:     "session-1",
				ClientID:      "client-1",
				UserID:        "user-1",
				RequestID:     "request-1",
				CorrelationID: "corr-1",
				MessageType:   "email_code",
				To:            "user@example.com",
				Data:          map[string]any{"name": "Alex"},
			}))
		})

		It("returns an error for invalid embedded data", func() {
			mapr := newMailMessageMapper()

			req, err := mapr.ToSendRequest(sendMailCommand{
				MessageType: "email_code",
				To:          "user@example.com",
				Data:        json.RawMessage(`{`),
			})

			Expect(req).To(Equal(mail.SendRequest{}))
			Expect(err).To(MatchError(ContainSubstring("unmarshal send mail command")))
		})
	})

	Describe("Result payloads", func() {
		It("encodes failure and success results", func() {
			mapr := newMailMessageMapper()

			failurePayload, err := mapr.ToFailureResultPayload(sendMailCommand{
				SessionID:   "session-1",
				MessageType: "email_code",
				To:          "user@example.com",
			}, "failed to send mail")
			Expect(err).NotTo(HaveOccurred())

			var failure sendMailResult
			Expect(json.Unmarshal(failurePayload, &failure)).To(Succeed())
			Expect(failure.Status).To(Equal("failed"))
			Expect(failure.Error).To(Equal("failed to send mail"))

			successPayload, err := mapr.ToSuccessResultPayload(&mail.SendResult{
				SessionID:   "session-1",
				MessageType: "email_code",
				To:          "user@example.com",
				Status:      "success",
			})
			Expect(err).NotTo(HaveOccurred())

			var success sendMailResult
			Expect(json.Unmarshal(successPayload, &success)).To(Succeed())
			Expect(success.Status).To(Equal("success"))
			Expect(success.To).To(Equal("user@example.com"))
		})
	})
})

var _ = Describe("FailureReasonResolver", func() {
	It("maps known domain errors to stable reasons", func() {
		resolver := NewDomainFailureReasonResolver()

		Expect(resolver.SendMailFailureReason(mail.ErrInvalidRecipientEmail)).To(Equal("invalid recipient email"))
		Expect(resolver.SendMailFailureReason(mail.ErrInvalidMessageType)).To(Equal("invalid message type"))
		Expect(resolver.SendMailFailureReason(mail.ErrTemplateNotFound)).To(Equal("mail template not found"))
		Expect(resolver.SendMailFailureReason(mail.ErrTemplateSubjectEmpty)).To(Equal("mail template subject is empty"))
		Expect(resolver.SendMailFailureReason(mail.ErrTemplateBodyEmpty)).To(Equal("mail template body is empty"))
		Expect(resolver.SendMailFailureReason(mail.ErrFailedToRenderMail)).To(Equal("failed to render mail template"))
		Expect(resolver.SendMailFailureReason(mail.ErrFailedToSendMail)).To(Equal("failed to send mail"))
		Expect(resolver.SendMailFailureReason(errors.New("boom"))).To(Equal("internal mail delivery error"))
	})

	It("maps grouped domain send errors to a stable application-level reason", func() {
		resolver := NewDomainFailureReasonResolver()

		err := errors.Join(mail.ErrTemplateNotFound, errors.New("wrapped"))

		Expect(resolver.SendMailFailureReason(err)).To(Equal("mail template not found"))
	})

})

var _ = Describe("BuildAdaptiveConfig", func() {
	It("projects adaptive consumer settings from config", func() {
		cfg := config.NATSConfig{
			MailAdaptiveEnabled:         true,
			MailAdaptiveCheckInterval:   2 * time.Second,
			MailAdaptiveMediumPending:   10,
			MailAdaptiveHighPending:     20,
			MailAdaptiveLowBatchSize:    1,
			MailAdaptiveLowMaxWait:      50 * time.Millisecond,
			MailAdaptiveMediumBatchSize: 5,
			MailAdaptiveMediumMaxWait:   10 * time.Millisecond,
			MailAdaptiveHighBatchSize:   10,
			MailAdaptiveHighMaxWait:     2 * time.Millisecond,
		}

		Expect(BuildAdaptiveConfig(cfg)).To(Equal(config.PullAdaptiveConfig{
			Enabled:         true,
			CheckInterval:   2 * time.Second,
			MediumPending:   10,
			HighPending:     20,
			LowBatchSize:    1,
			LowMaxWait:      50 * time.Millisecond,
			MediumBatchSize: 5,
			MediumMaxWait:   10 * time.Millisecond,
			HighBatchSize:   10,
			HighMaxWait:     2 * time.Millisecond,
		}))
	})
})

var _ = Describe("pullConsumerRuntime helpers", func() {
	var runtime *pullConsumerRuntime

	BeforeEach(func() {
		lg, err := logging.New("mail-service", "test", "debug")
		Expect(err).NotTo(HaveOccurred())
		runtime = &pullConsumerRuntime{
			log: lg,
			cfg: config.PullConsumerConfig{
				Subject: "mail.send",
				Durable: "mail_service_send",
			},
			jobs: make(chan *nats.Msg, 2),
		}
	})

	It("treats timeout-like fetch errors as retryable without logging a failure", func() {
		Expect(runtime.handleFetchError(nats.ErrTimeout)).To(BeTrue())
		Expect(runtime.handleFetchError(context.DeadlineExceeded)).To(BeTrue())
		Expect(runtime.handleFetchError(nil)).To(BeFalse())
	})

	It("dispatches fetched messages until context cancellation", func() {
		msgs := []*nats.Msg{{Subject: "one"}, {Subject: "two"}}
		ctx := context.Background()

		Expect(runtime.dispatchFetchedMessages(ctx, msgs)).To(BeFalse())
		Expect(runtime.jobs).To(HaveLen(2))

		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		Expect(runtime.dispatchFetchedMessages(cancelCtx, msgs)).To(BeTrue())
	})

	It("runs a worker that naks handler failures", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		var wg sync.WaitGroup
		runtime.handler = func(context.Context, string, []byte) error {
			return errors.New("boom")
		}

		wg.Add(1)
		go runtime.runWorker(ctx, &wg, 1)
		runtime.jobs <- &nats.Msg{Subject: "mail.send", Data: []byte("payload")}
		close(runtime.jobs)
		wg.Wait()
	})

	It("runs a worker that acknowledges successful messages", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		var wg sync.WaitGroup
		runtime.handler = func(context.Context, string, []byte) error {
			return nil
		}

		wg.Add(1)
		go runtime.runWorker(ctx, &wg, 1)
		runtime.jobs <- &nats.Msg{Subject: "mail.send", Data: []byte("payload")}
		close(runtime.jobs)
		wg.Wait()
	})

})

var _ = Describe("NATSBroker helpers", func() {
	var logger logging.Logger

	BeforeEach(func() {
		var err error
		logger, err = logging.New("mail-service", "test", "debug")
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("NewBroker", func() {
		It("validates config and logger", func() {
			broker, err := NewBroker(config.NATSConfig{}, logger)
			Expect(broker).To(BeNil())
			Expect(err).To(MatchError(ErrEmptyNATSURL))

			broker, err = NewBroker(config.NATSConfig{URL: "nats://127.0.0.1:4222"}, nil)
			Expect(broker).To(BeNil())
			Expect(err).To(MatchError(ErrNilLogger))
		})
	})

	Describe("RunPullConsumer", func() {
		var broker *natsBroker

		BeforeEach(func() {
			broker = &natsBroker{log: logger}
		})

		It("validates required consumer fields before touching NATS", func() {
			err := broker.RunPullConsumer(context.Background(), config.PullConsumerConfig{}, nil)
			Expect(err).To(MatchError(ErrEmptyStreamName))

			err = broker.RunPullConsumer(context.Background(), config.PullConsumerConfig{
				Stream: "MAIL_COMMANDS",
			}, nil)
			Expect(err).To(MatchError(ErrEmptySubject))

			err = broker.RunPullConsumer(context.Background(), config.PullConsumerConfig{
				Stream:  "MAIL_COMMANDS",
				Subject: "mail.send",
			}, nil)
			Expect(err).To(MatchError(ErrEmptyDurableName))
		})

		It("validates worker and batch settings before touching NATS", func() {
			err := broker.RunPullConsumer(context.Background(), config.PullConsumerConfig{
				Stream:  "MAIL_COMMANDS",
				Subject: "mail.send",
				Durable: "mail_service_send",
			}, nil)
			Expect(err).To(MatchError(ErrInvalidBatchSize))

			err = broker.RunPullConsumer(context.Background(), config.PullConsumerConfig{
				Stream:    "MAIL_COMMANDS",
				Subject:   "mail.send",
				Durable:   "mail_service_send",
				BatchSize: 1,
			}, nil)
			Expect(err).To(MatchError(ErrInvalidMaxWait))

			err = broker.RunPullConsumer(context.Background(), config.PullConsumerConfig{
				Stream:    "MAIL_COMMANDS",
				Subject:   "mail.send",
				Durable:   "mail_service_send",
				BatchSize: 1,
				MaxWait:   time.Millisecond,
			}, nil)
			Expect(err).To(MatchError(ErrInvalidWorkerCount))

			err = broker.RunPullConsumer(context.Background(), config.PullConsumerConfig{
				Stream:    "MAIL_COMMANDS",
				Subject:   "mail.send",
				Durable:   "mail_service_send",
				BatchSize: 1,
				MaxWait:   time.Millisecond,
				Workers:   1,
			}, nil)
			Expect(err).To(MatchError(ErrInvalidQueueSize))

			err = broker.RunPullConsumer(context.Background(), config.PullConsumerConfig{
				Stream:    "MAIL_COMMANDS",
				Subject:   "mail.send",
				Durable:   "mail_service_send",
				BatchSize: 1,
				MaxWait:   time.Millisecond,
				Workers:   1,
				QueueSize: 1,
			}, nil)
			Expect(err).To(MatchError(ErrInvalidAckWait))

			err = broker.RunPullConsumer(context.Background(), config.PullConsumerConfig{
				Stream:    "MAIL_COMMANDS",
				Subject:   "mail.send",
				Durable:   "mail_service_send",
				BatchSize: 1,
				MaxWait:   time.Millisecond,
				Workers:   1,
				QueueSize: 1,
				AckWait:   time.Second,
			}, nil)
			Expect(err).To(MatchError(ErrInvalidMaxDeliver))
		})

		It("validates adaptive settings before touching NATS", func() {
			base := config.PullConsumerConfig{
				Stream:     "MAIL_COMMANDS",
				Subject:    "mail.send",
				Durable:    "mail_service_send",
				BatchSize:  1,
				MaxWait:    time.Millisecond,
				Workers:    1,
				QueueSize:  1,
				AckWait:    time.Second,
				MaxDeliver: 1,
				Adaptive: config.PullAdaptiveConfig{
					Enabled: true,
				},
			}

			err := broker.RunPullConsumer(context.Background(), base, nil)
			Expect(err).To(MatchError(ErrInvalidAdaptiveCheckInterval))

			base.Adaptive.CheckInterval = time.Second
			err = broker.RunPullConsumer(context.Background(), base, nil)
			Expect(err).To(MatchError(ErrInvalidAdaptiveThresholds))

			base.Adaptive.MediumPending = 1
			base.Adaptive.HighPending = 2
			err = broker.RunPullConsumer(context.Background(), base, nil)
			Expect(err).To(MatchError(ErrInvalidAdaptivePlan))
		})

		It("returns nil when adaptive mode is disabled", func() {
			runtime := &stubPullConsumerRuntime{}
			broker.runtimeFactory = &stubPullConsumerRuntimeFactory{runtime: runtime}

			err := broker.RunPullConsumer(context.Background(), config.PullConsumerConfig{
				Stream:     "MAIL_COMMANDS",
				Subject:    "mail.send",
				Durable:    "mail_service_send",
				BatchSize:  1,
				MaxWait:    time.Millisecond,
				Workers:    1,
				QueueSize:  1,
				AckWait:    time.Second,
				MaxDeliver: 1,
			}, nil)

			Expect(err).NotTo(HaveOccurred())
			Expect(runtime.started).To(BeTrue())
		})
	})

	Describe("ResolvePullPlan", func() {
		It("returns the configured tier plan", func() {
			cfg := config.PullConsumerConfig{
				BatchSize: 10,
				MaxWait:   10 * time.Millisecond,
				Adaptive: config.PullAdaptiveConfig{
					Enabled:         true,
					MediumPending:   10,
					HighPending:     20,
					LowBatchSize:    1,
					LowMaxWait:      50 * time.Millisecond,
					MediumBatchSize: 5,
					MediumMaxWait:   10 * time.Millisecond,
					HighBatchSize:   10,
					HighMaxWait:     2 * time.Millisecond,
				},
			}

			tier, batch, wait := ResolvePullPlan(cfg, 25)
			Expect(tier).To(Equal("high"))
			Expect(batch).To(Equal(10))
			Expect(wait).To(Equal(2 * time.Millisecond))

			tier, batch, wait = ResolvePullPlan(cfg, 15)
			Expect(tier).To(Equal("medium"))
			Expect(batch).To(Equal(5))
			Expect(wait).To(Equal(10 * time.Millisecond))

			tier, batch, wait = ResolvePullPlan(cfg, 1)
			Expect(tier).To(Equal("low"))
			Expect(batch).To(Equal(1))
			Expect(wait).To(Equal(50 * time.Millisecond))

			cfg.Adaptive.Enabled = false
			tier, batch, wait = ResolvePullPlan(cfg, 25)
			Expect(tier).To(Equal("base"))
			Expect(batch).To(Equal(10))
			Expect(wait).To(Equal(10 * time.Millisecond))
		})
	})

	Describe("Error wrappers", func() {
		It("annotates transport failures", func() {
			Expect(WrapConnectToNATSError(errors.New("boom"))).To(MatchError(ContainSubstring("connect to nats")))
			Expect(WrapPublishToNATSError("mail.send", errors.New("boom"))).To(MatchError(ContainSubstring("publish to nats (mail.send)")))
			Expect(WrapSubscribeToNATSError("mail.send", errors.New("boom"))).To(MatchError(ContainSubstring("subscribe to nats (mail.send)")))
			Expect(WrapFlushNATSPublisherError(errors.New("boom"))).To(MatchError(ContainSubstring("flush nats publisher")))
			Expect(WrapInitJetStreamContextError(errors.New("boom"))).To(MatchError(ContainSubstring("init jetstream context")))
			Expect(WrapEnsureConsumerError("MAIL_COMMANDS", "mail_service_send", errors.New("add"), errors.New("update"))).
				To(MatchError(ContainSubstring(`ensure consumer "mail_service_send" in stream "MAIL_COMMANDS"`)))
			Expect(WrapCreatePullSubscriberError("mail.send", "mail_service_send", errors.New("boom"))).
				To(MatchError(ContainSubstring(`create pull subscriber subject="mail.send" durable="mail_service_send"`)))
			Expect(WrapUnmarshalSendMailCommandError(errors.New("boom"))).To(MatchError(ContainSubstring("unmarshal send mail command")))
			Expect(WrapMarshalSendMailResultError(errors.New("boom"))).To(MatchError(ContainSubstring("marshal send mail result")))
		})
	})
})

type MockEventBroker struct {
	ctrl     *gomock.Controller
	recorder *MockEventBrokerMockRecorder
}

type MockEventBrokerMockRecorder struct {
	mock *MockEventBroker
}

func NewMockEventBroker(ctrl *gomock.Controller) *MockEventBroker {
	mock := &MockEventBroker{ctrl: ctrl}
	mock.recorder = &MockEventBrokerMockRecorder{mock}
	return mock
}

func (m *MockEventBroker) EXPECT() *MockEventBrokerMockRecorder {
	return m.recorder
}

func (m *MockEventBroker) Publish(ctx context.Context, subject string, payload []byte) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Publish", ctx, subject, payload)
	ret0, _ := ret[0].(error)
	return ret0
}

func (mr *MockEventBrokerMockRecorder) Publish(ctx, subject, payload any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Publish", reflect.TypeOf((*MockEventBroker)(nil).Publish), ctx, subject, payload)
}

func (m *MockEventBroker) Subscribe(ctx context.Context, subject string, handler eventbroker.MessageHandler) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Subscribe", ctx, subject, handler)
	ret0, _ := ret[0].(error)
	return ret0
}

func (mr *MockEventBrokerMockRecorder) Subscribe(ctx, subject, handler any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Subscribe", reflect.TypeOf((*MockEventBroker)(nil).Subscribe), ctx, subject, handler)
}

func (m *MockEventBroker) RunPullConsumer(ctx context.Context, cfg config.PullConsumerConfig, handler eventbroker.MessageHandler) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RunPullConsumer", ctx, cfg, handler)
	ret0, _ := ret[0].(error)
	return ret0
}

func (mr *MockEventBrokerMockRecorder) RunPullConsumer(ctx, cfg, handler any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RunPullConsumer", reflect.TypeOf((*MockEventBroker)(nil).RunPullConsumer), ctx, cfg, handler)
}

func (m *MockEventBroker) Close() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Close")
}

func (mr *MockEventBrokerMockRecorder) Close() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*MockEventBroker)(nil).Close))
}

type MockMailService struct {
	ctrl     *gomock.Controller
	recorder *MockMailServiceMockRecorder
}

type MockMailServiceMockRecorder struct {
	mock *MockMailService
}

func NewMockMailService(ctrl *gomock.Controller) *MockMailService {
	mock := &MockMailService{ctrl: ctrl}
	mock.recorder = &MockMailServiceMockRecorder{mock}
	return mock
}

func (m *MockMailService) EXPECT() *MockMailServiceMockRecorder {
	return m.recorder
}

func (m *MockMailService) Send(ctx context.Context, req mail.SendRequest) (*mail.SendResult, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Send", ctx, req)
	ret0, _ := ret[0].(*mail.SendResult)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (mr *MockMailServiceMockRecorder) Send(ctx, req any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Send", reflect.TypeOf((*MockMailService)(nil).Send), ctx, req)
}
