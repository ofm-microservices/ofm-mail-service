package nats

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/ofm-microseervices/ofm-common/pkg/logging"
	"mail-service/config"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

var (
	natsSuiteContainer testcontainers.Container
	natsSuiteBaseCfg   config.NATSConfig
)

var _ = BeforeSuite(func() {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	natsSuiteContainer, natsSuiteBaseCfg = startNATSContainer(ctx)
})

var _ = AfterSuite(func() {
	if natsSuiteContainer != nil {
		Expect(natsSuiteContainer.Terminate(context.Background())).To(Succeed())
	}
})

var _ = Describe("natsBroker integration", func() {
	var (
		ctx    context.Context
		cancel context.CancelFunc
		cfg    config.NATSConfig
		lg     logging.Logger
	)

	BeforeEach(func() {
		ctx, cancel = context.WithTimeout(context.Background(), 45*time.Second)
		cfg = uniqueNATSConfig(natsSuiteBaseCfg)

		var err error
		lg, err = logging.New("mail-service", "test", "debug")
		Expect(err).NotTo(HaveOccurred())

		bootstrapConn, err := nats.Connect(cfg.URL)
		Expect(err).NotTo(HaveOccurred())
		defer bootstrapConn.Close()

		js, err := bootstrapConn.JetStream()
		Expect(err).NotTo(HaveOccurred())
		_, err = js.AddStream(&nats.StreamConfig{
			Name:      cfg.MailCommandsStream,
			Subjects:  []string{cfg.MailSendSubject},
			Storage:   nats.FileStorage,
			Retention: nats.LimitsPolicy,
		})
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		cancel()
	})

	It("publishes messages to core nats subjects", func() {
		rawConn, err := nats.Connect(cfg.URL)
		Expect(err).NotTo(HaveOccurred())
		defer rawConn.Close()

		sub, err := rawConn.SubscribeSync("mail.send.result")
		Expect(err).NotTo(HaveOccurred())
		Expect(rawConn.Flush()).To(Succeed())

		broker, err := NewBroker(cfg, lg)
		Expect(err).NotTo(HaveOccurred())
		defer broker.Close()

		Expect(broker.Publish(ctx, "mail.send.result", []byte("payload"))).To(Succeed())

		msg, err := sub.NextMsg(5 * time.Second)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(msg.Data)).To(Equal("payload"))
	})

	It("flushes with a synthetic timeout when the caller provides no deadline", func() {
		rawConn, err := nats.Connect(cfg.URL)
		Expect(err).NotTo(HaveOccurred())
		defer rawConn.Close()

		Expect(Flush(context.Background(), rawConn)).To(Succeed())
	})

	It("subscribes and dispatches core nats messages", func() {
		rawConn, err := nats.Connect(cfg.URL)
		Expect(err).NotTo(HaveOccurred())
		defer rawConn.Close()

		broker, err := NewBroker(cfg, lg)
		Expect(err).NotTo(HaveOccurred())
		defer broker.Close()

		received := make(chan []byte, 1)
		Expect(broker.Subscribe(ctx, "mail.send.result", func(_ context.Context, subject string, payload []byte) error {
			Expect(subject).To(Equal("mail.send.result"))
			received <- payload
			return nil
		})).To(Succeed())

		Expect(rawConn.Publish("mail.send.result", []byte("payload"))).To(Succeed())
		Expect(rawConn.Flush()).To(Succeed())

		Eventually(received).Should(Receive(Equal([]byte("payload"))))
	})

	It("wraps publish failures on a closed connection", func() {
		broker, err := NewBroker(cfg, lg)
		Expect(err).NotTo(HaveOccurred())
		concrete := broker.(*natsBroker)
		concrete.Close()

		err = concrete.Publish(ctx, "mail.send.result", []byte("payload"))

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("publish to nats"))
	})

	It("wraps subscribe failures on a closed connection", func() {
		broker, err := NewBroker(cfg, lg)
		Expect(err).NotTo(HaveOccurred())
		concrete := broker.(*natsBroker)
		concrete.Close()

		err = concrete.Subscribe(ctx, "mail.send.result", func(context.Context, string, []byte) error {
			return nil
		})

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("subscribe to nats"))
	})

	It("runs a pull consumer against jetstream", func() {
		rawConn, err := nats.Connect(cfg.URL)
		Expect(err).NotTo(HaveOccurred())
		defer rawConn.Close()

		broker, err := NewBroker(cfg, lg)
		Expect(err).NotTo(HaveOccurred())
		concrete := broker.(*natsBroker)
		defer concrete.Close()

		runCtx, runCancel := context.WithCancel(ctx)
		defer runCancel()

		var handled atomic.Int32
		Expect(concrete.RunPullConsumer(runCtx, config.PullConsumerConfig{
			Stream:     cfg.MailCommandsStream,
			Subject:    cfg.MailSendSubject,
			Durable:    cfg.MailSendDurable,
			BatchSize:  2,
			MaxWait:    50 * time.Millisecond,
			Workers:    1,
			QueueSize:  4,
			AckWait:    2 * time.Second,
			MaxDeliver: 3,
			Adaptive: config.PullAdaptiveConfig{
				Enabled:         true,
				CheckInterval:   1 * time.Millisecond,
				MediumPending:   1,
				HighPending:     2,
				LowBatchSize:    1,
				LowMaxWait:      20 * time.Millisecond,
				MediumBatchSize: 2,
				MediumMaxWait:   10 * time.Millisecond,
				HighBatchSize:   3,
				HighMaxWait:     5 * time.Millisecond,
			},
		}, func(_ context.Context, subject string, payload []byte) error {
			Expect(subject).To(Equal(cfg.MailSendSubject))
			Expect(payload).NotTo(BeEmpty())
			handled.Add(1)
			return nil
		})).To(Succeed())

		Expect(rawConn.Publish(cfg.MailSendSubject, []byte("one"))).To(Succeed())
		Expect(rawConn.Publish(cfg.MailSendSubject, []byte("two"))).To(Succeed())
		Expect(rawConn.Publish(cfg.MailSendSubject, []byte("three"))).To(Succeed())
		Expect(rawConn.Flush()).To(Succeed())

		Eventually(func() int32 { return handled.Load() }).Should(Equal(int32(3)))
		runCancel()
	})

	It("switches the adaptive pull plan when the pending backlog grows", func() {
		rawConn, err := nats.Connect(cfg.URL)
		Expect(err).NotTo(HaveOccurred())
		defer rawConn.Close()

		broker, err := NewBroker(cfg, lg)
		Expect(err).NotTo(HaveOccurred())
		concrete := broker.(*natsBroker)
		defer concrete.Close()

		handler := func(context.Context, string, []byte) error { return nil }
		cfg := config.PullConsumerConfig{
			Stream:     cfg.MailCommandsStream,
			Subject:    cfg.MailSendSubject,
			Durable:    cfg.MailSendDurable + "_adaptive",
			BatchSize:  1,
			MaxWait:    50 * time.Millisecond,
			Workers:    1,
			QueueSize:  4,
			AckWait:    2 * time.Second,
			MaxDeliver: 3,
			Adaptive: config.PullAdaptiveConfig{
				Enabled:         true,
				CheckInterval:   1 * time.Millisecond,
				MediumPending:   1,
				HighPending:     2,
				LowBatchSize:    1,
				LowMaxWait:      20 * time.Millisecond,
				MediumBatchSize: 2,
				MediumMaxWait:   10 * time.Millisecond,
				HighBatchSize:   3,
				HighMaxWait:     5 * time.Millisecond,
			},
		}

		runtimeAny, err := concrete.runtimeFactory.Create(concrete.nc, concrete.log, cfg, handler)
		Expect(err).NotTo(HaveOccurred())
		runtime := runtimeAny.(*pullConsumerRuntime)

		Expect(rawConn.Publish(cfg.Subject, []byte("one"))).To(Succeed())
		Expect(rawConn.Publish(cfg.Subject, []byte("two"))).To(Succeed())
		Expect(rawConn.Publish(cfg.Subject, []byte("three"))).To(Succeed())
		Expect(rawConn.Flush()).To(Succeed())

		state := pullConsumerFetchState{
			batch:             cfg.BatchSize,
			wait:              cfg.MaxWait,
			tier:              "base",
			lastAdaptiveCheck: time.Now().Add(-time.Second),
		}

		Eventually(func() string {
			runtime.maybeUpdateAdaptivePlan(&state)
			return state.tier
		}).Should(Equal("high"))
		Expect(state.batch).To(Equal(3))
		Expect(state.wait).To(Equal(5 * time.Millisecond))
	})

	It("closes the underlying nats connection", func() {
		broker, err := NewBroker(cfg, lg)
		Expect(err).NotTo(HaveOccurred())
		concrete := broker.(*natsBroker)

		concrete.Close()

		Expect(concrete.nc.IsClosed()).To(BeTrue())
	})
})

func startNATSContainer(ctx context.Context) (testcontainers.Container, config.NATSConfig) {
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

	return container, config.NATSConfig{
		URL:                   "nats://" + host + ":" + port.Port(),
		MailCommandsStream:    "MAIL_COMMANDS",
		MailEventsStream:      "MAIL_EVENTS",
		MailSendSubject:       "mail.send",
		MailSendResultSubject: "mail.send.result",
		MailSendDurable:       "mail_service_send",
	}
}

func uniqueNATSConfig(base config.NATSConfig) config.NATSConfig {
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	base.MailCommandsStream = base.MailCommandsStream + "_" + suffix
	base.MailEventsStream = base.MailEventsStream + "_" + suffix
	base.MailSendSubject = base.MailSendSubject + "." + suffix
	base.MailSendResultSubject = base.MailSendResultSubject + "." + suffix
	base.MailSendDurable = base.MailSendDurable + "_" + suffix
	return base
}
