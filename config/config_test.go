package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"mail-service/config"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestConfig(t *testing.T) {
	t.Helper()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Config Suite")
}

var _ = Describe("Load", func() {
	var prevWD string

	BeforeEach(func() {
		var err error
		prevWD, err = os.Getwd()
		Expect(err).NotTo(HaveOccurred())
		Expect(os.Unsetenv("APP_ENV")).To(Succeed())
		Expect(os.Unsetenv("LOG_LEVEL")).To(Succeed())
		Expect(os.Unsetenv("NATS_URL")).To(Succeed())
		Expect(os.Unsetenv("NATS_USER")).To(Succeed())
		Expect(os.Unsetenv("NATS_PASSWORD")).To(Succeed())
		Expect(os.Unsetenv("SENDER_EMAIL")).To(Succeed())
		Expect(os.Unsetenv("EMAIL_PASSWORD")).To(Succeed())
		Expect(os.Unsetenv("MAIL_TEMPLATE_DIR")).To(Succeed())
	})

	AfterEach(func() {
		Expect(os.Chdir(prevWD)).To(Succeed())
	})

	It("loads defaults and required values from the environment", func() {
		Expect(os.Setenv("NATS_URL", "nats://localhost:4222")).To(Succeed())
		Expect(os.Setenv("SENDER_EMAIL", "mailer@example.com")).To(Succeed())
		Expect(os.Setenv("EMAIL_PASSWORD", "secret")).To(Succeed())

		cfg, err := config.Load()

		Expect(err).NotTo(HaveOccurred())
		Expect(cfg.App.Env).To(Equal("local"))
		Expect(cfg.App.LogLevel).To(Equal("info"))
		Expect(cfg.SMTP.Host).To(Equal("smtp.gmail.com"))
		Expect(cfg.SMTP.Port).To(Equal(587))
		Expect(cfg.SMTP.Mode).To(Equal("starttls"))
		Expect(cfg.SMTP.SenderEmail).To(Equal("mailer@example.com"))
		Expect(cfg.SMTP.Password).To(Equal("secret"))
		Expect(cfg.SMTP.FromName).To(Equal("OFM"))
		Expect(cfg.NATS.URL).To(Equal("nats://localhost:4222"))
		Expect(cfg.NATS.MailCommandsStream).To(Equal("MAIL_COMMANDS"))
		Expect(cfg.NATS.MailEventsStream).To(Equal("MAIL_EVENTS"))
		Expect(cfg.NATS.MailBatchSize).To(Equal(32))
		Expect(cfg.NATS.MailMaxWait).To(Equal(10 * time.Millisecond))
		Expect(cfg.NATS.MailWorkers).To(Equal(8))
		Expect(cfg.NATS.MailQueueSize).To(Equal(500))
		Expect(cfg.NATS.MailAckWait).To(Equal(30 * time.Second))
		Expect(cfg.NATS.MailAdaptiveEnabled).To(BeFalse())
		Expect(cfg.Templates.Dir).To(Equal("templates"))
	})

	It("loads values from a local .env file before parsing", func() {
		tmpDir := GinkgoT().TempDir()
		envFile := []byte("NATS_URL=nats://dotenv:4222\nSENDER_EMAIL=dotenv@example.com\nEMAIL_PASSWORD=dotenv-secret\nAPP_ENV=dev\nLOG_LEVEL=debug\n")
		Expect(os.WriteFile(filepath.Join(tmpDir, ".env"), envFile, 0o600)).To(Succeed())
		Expect(os.Chdir(tmpDir)).To(Succeed())

		cfg, err := config.Load()

		Expect(err).NotTo(HaveOccurred())
		Expect(cfg.App.Env).To(Equal("dev"))
		Expect(cfg.App.LogLevel).To(Equal("debug"))
		Expect(cfg.NATS.URL).To(Equal("nats://dotenv:4222"))
		Expect(cfg.SMTP.SenderEmail).To(Equal("dotenv@example.com"))
		Expect(cfg.SMTP.Password).To(Equal("dotenv-secret"))
	})

	It("wraps env parse errors", func() {
		Expect(os.Setenv("NATS_URL", "nats://localhost:4222")).To(Succeed())
		Expect(os.Setenv("SENDER_EMAIL", "mailer@example.com")).To(Succeed())
		Expect(os.Setenv("EMAIL_PASSWORD", "secret")).To(Succeed())
		Expect(os.Setenv("SMTP_PORT", "bad-port")).To(Succeed())

		_, err := config.Load()

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("parse env config"))
		Expect(errors.Is(err, config.WrapParseEnvConfigError(errors.New("x")))).To(BeFalse())
	})
})
