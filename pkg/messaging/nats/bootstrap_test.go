package nats_test

import (
	"errors"
	"testing"

	"github.com/ofm-microseervices/ofm-common/pkg/logging"
	"mail-service/config"
	natsbootstrap "mail-service/pkg/messaging/nats"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestBootstrap(t *testing.T) {
	t.Helper()
	RegisterFailHandler(Fail)
	RunSpecs(t, "NATS Bootstrap Suite")
}

var _ = Describe("Connect", func() {
	It("wraps connection failures", func() {
		nc, err := natsbootstrap.Connect(config.NATSConfig{URL: "://bad-url"})

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("connect to nats"))
		Expect(nc).To(BeNil())
	})
})

var _ = Describe("EnsureStream", func() {
	It("requires a logger", func() {
		err := natsbootstrap.EnsureStream(config.NATSConfig{}, nil)

		Expect(err).To(MatchError(natsbootstrap.ErrNilLogger))
	})

	It("returns the connection error when nats is unreachable", func() {
		lg, err := logging.New("mail-service", "test", "debug")
		Expect(err).NotTo(HaveOccurred())

		err = natsbootstrap.EnsureStream(config.NATSConfig{
			URL:                   "nats://127.0.0.1:1",
			MailCommandsStream:    "MAIL_COMMANDS",
			MailEventsStream:      "MAIL_EVENTS",
			MailSendSubject:       "mail.send",
			MailSendResultSubject: "mail.send.result",
		}, lg)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("connect to nats"))
	})
})

var _ = Describe("error wrappers", func() {
	It("wraps bootstrap errors", func() {
		base := errors.New("boom")

		Expect(natsbootstrap.WrapConnectToNATSError(base).Error()).To(ContainSubstring("connect to nats"))
		Expect(natsbootstrap.WrapInitJetStreamContextError(base).Error()).To(ContainSubstring("init jetstream context"))
		Expect(natsbootstrap.WrapEnsureStreamError("MAIL_EVENTS", base, base).Error()).To(ContainSubstring(`ensure stream "MAIL_EVENTS"`))
	})
})
