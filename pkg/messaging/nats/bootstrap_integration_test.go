package nats_test

import (
	"context"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/ofm-microservices/ofm-common/pkg/logging"
	"mail-service/config"
	natsbootstrap "mail-service/pkg/messaging/nats"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

var _ = Describe("bootstrap integration", func() {
	It("connects to a real nats server", func() {
		nc, err := natsbootstrap.Connect(cfg)

		Expect(err).NotTo(HaveOccurred())
		Expect(nc).NotTo(BeNil())
		Expect(nc.IsConnected()).To(BeTrue())
		nc.Close()
	})

	It("creates and then updates the required streams", func() {
		Expect(natsbootstrap.EnsureStream(cfg, lg)).To(Succeed())
		Expect(natsbootstrap.EnsureStream(cfg, lg)).To(Succeed())

		nc, err := nats.Connect(cfg.URL)
		Expect(err).NotTo(HaveOccurred())
		defer nc.Close()

		js, err := nc.JetStream()
		Expect(err).NotTo(HaveOccurred())

		mailCommandsInfo, err := js.StreamInfo(cfg.MailCommandsStream)
		Expect(err).NotTo(HaveOccurred())
		Expect(mailCommandsInfo.Config.Subjects).To(ContainElement(cfg.MailSendSubject))

		mailEventsInfo, err := js.StreamInfo(cfg.MailEventsStream)
		Expect(err).NotTo(HaveOccurred())
		Expect(mailEventsInfo.Config.Subjects).To(ContainElement(cfg.MailSendResultSubject))
	})

	It("wraps jetstream initialization failures when the server has no jetstream", func() {
		err := natsbootstrap.EnsureStream(bootstrapNoJSCfg, lg)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(`ensure stream "MAIL_COMMANDS"`))
	})
})

var (
	bootstrapJSContainer   testcontainers.Container
	bootstrapNoJSContainer testcontainers.Container
	bootstrapJSCfg         config.NATSConfig
	bootstrapNoJSCfg       config.NATSConfig
	ctx                    context.Context
	cancel                 context.CancelFunc
	cfg                    config.NATSConfig
	lg                     logging.Logger
)

var _ = BeforeSuite(func() {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	bootstrapJSContainer, bootstrapJSCfg = startBootstrapNATSContainer(ctx, true)
	bootstrapNoJSContainer, bootstrapNoJSCfg = startBootstrapNATSContainer(ctx, false)
})

var _ = AfterSuite(func() {
	if bootstrapJSContainer != nil {
		Expect(bootstrapJSContainer.Terminate(context.Background())).To(Succeed())
	}
	if bootstrapNoJSContainer != nil {
		Expect(bootstrapNoJSContainer.Terminate(context.Background())).To(Succeed())
	}
})

var _ = BeforeEach(func() {
	ctx, cancel = context.WithTimeout(context.Background(), 45*time.Second)
	cfg = bootstrapJSCfg

	var err error
	lg, err = logging.New("mail-service", "test", "debug")
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterEach(func() {
	cancel()
})

func startBootstrapNATSContainer(ctx context.Context, jetstream bool) (testcontainers.Container, config.NATSConfig) {
	cmd := []string{}
	if jetstream {
		cmd = []string{"-js"}
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "nats:2.11-alpine",
			ExposedPorts: []string{"4222/tcp"},
			Cmd:          cmd,
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
	}
}
