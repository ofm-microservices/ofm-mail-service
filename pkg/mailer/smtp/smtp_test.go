package smtp

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/ofm-microseervices/ofm-common/pkg/logging"
	mail "mail-service/internal/domain"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestSMTP(t *testing.T) {
	t.Helper()
	RegisterFailHandler(Fail)
	RunSpecs(t, "SMTP Suite")
}

var _ = Describe("New", func() {
	var (
		cfg SMTPConfig
		lg  logging.Logger
	)

	BeforeEach(func() {
		var err error
		lg, err = logging.New("mail-service", "test", "debug")
		Expect(err).NotTo(HaveOccurred())
		cfg = SMTPConfig{
			Host:        "127.0.0.1",
			Port:        2525,
			Mode:        "starttls",
			SenderEmail: "mailer@example.com",
			Password:    "secret",
			FromName:    "OFM",
		}
	})

	It("validates required fields", func() {
		cfg.Host = ""
		sender, err := New(cfg, lg)
		Expect(err).To(MatchError(ErrEmptyHost))
		Expect(sender).To(BeNil())

		cfg.Host = "127.0.0.1"
		cfg.Port = 0
		sender, err = New(cfg, lg)
		Expect(err).To(MatchError(ErrInvalidPort))
		Expect(sender).To(BeNil())

		cfg.Port = 2525
		cfg.SenderEmail = ""
		sender, err = New(cfg, lg)
		Expect(err).To(MatchError(ErrEmptySenderEmail))
		Expect(sender).To(BeNil())

		cfg.SenderEmail = "mailer@example.com"
		cfg.Password = ""
		sender, err = New(cfg, lg)
		Expect(err).To(MatchError(ErrEmptyPassword))
		Expect(sender).To(BeNil())

		cfg.Password = "secret"
		cfg.Mode = "plain"
		sender, err = New(cfg, lg)
		Expect(err).To(MatchError(ErrInvalidMode))
		Expect(sender).To(BeNil())

		cfg.Mode = "starttls"
		sender, err = New(cfg, nil)
		Expect(err).To(MatchError(ErrNilLogger))
		Expect(sender).To(BeNil())
	})

	It("formats the display address when a from name is configured", func() {
		sender, err := New(cfg, lg)

		Expect(err).NotTo(HaveOccurred())
		concrete, ok := sender.(*smtpSender)
		Expect(ok).To(BeTrue())
		Expect(concrete.from).To(ContainSubstring("OFM"))
		Expect(concrete.fromAddr).To(Equal("mailer@example.com"))
	})

	It("uses the raw sender address when no display name is configured", func() {
		cfg.FromName = ""

		sender, err := New(cfg, lg)

		Expect(err).NotTo(HaveOccurred())
		concrete, ok := sender.(*smtpSender)
		Expect(ok).To(BeTrue())
		Expect(concrete.from).To(Equal("mailer@example.com"))
	})
})

var _ = Describe("smtpSender", func() {
	var sender *smtpSender

	BeforeEach(func() {
		lg, err := logging.New("mail-service", "test", "debug")
		Expect(err).NotTo(HaveOccurred())
		sender = &smtpSender{
			cfg: SMTPConfig{
				Host:        "127.0.0.1",
				Port:        2525,
				Mode:        "starttls",
				SenderEmail: "mailer@example.com",
				Password:    "secret",
			},
			from:     "OFM <mailer@example.com>",
			fromAddr: "mailer@example.com",
			log:      lg,
		}
	})

	It("rejects invalid recipient email before dialing smtp", func() {
		err := sender.Send(context.Background(), mail.Email{
			To:       "bad-address",
			Subject:  "Subject",
			HTMLBody: "<p>Hello</p>",
		})

		Expect(err).To(MatchError(mail.ErrInvalidRecipientEmail))
	})

	It("wraps dial failures from the smtp client creation path", func() {
		sender.cfg.Port = 1

		err := sender.Send(context.Background(), mail.Email{
			To:       "user@example.com",
			Subject:  "Subject",
			HTMLBody: "<p>Hello</p>",
		})

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(mail.ErrFailedToSendMail.Error()))
		Expect(err.Error()).To(ContainSubstring("dial smtp"))
	})

	It("builds an html smtp message", func() {
		msg := sender.buildMessage(mail.Email{
			To:       "user@example.com",
			Subject:  "Welcome",
			HTMLBody: "<p>Hello</p>",
		})

		Expect(msg).To(ContainSubstring("From: OFM <mailer@example.com>"))
		Expect(msg).To(ContainSubstring("To: user@example.com"))
		Expect(msg).To(ContainSubstring("Subject: Welcome"))
		Expect(msg).To(ContainSubstring(`Content-Type: text/html; charset="UTF-8"`))
		Expect(msg).To(ContainSubstring("<p>Hello</p>"))
	})

	It("sends a message through a tls smtp server", func() {
		server, err := startTLSSMTPServer(smtpServerBehavior{})
		Expect(err).NotTo(HaveOccurred())
		defer server.Close()
		allowInsecureTLSTestDial()

		lg, err := logging.New("mail-service", "test", "debug")
		Expect(err).NotTo(HaveOccurred())

		sender, err := New(SMTPConfig{
			Host:        server.host,
			Port:        server.port,
			Mode:        "tls",
			SenderEmail: "mailer@example.com",
			Password:    "secret",
			FromName:    "OFM",
		}, lg)
		Expect(err).NotTo(HaveOccurred())

		err = sender.Send(context.Background(), mail.Email{
			To:       "user@example.com",
			Subject:  "Welcome",
			HTMLBody: "<p>Hello</p>",
		})

		Expect(err).NotTo(HaveOccurred())
		Eventually(server.payload).Should(Receive(ContainSubstring("<p>Hello</p>")))
	})

	It("wraps smtp authentication failures", func() {
		server, err := startTLSSMTPServer(smtpServerBehavior{authResponse: "535 5.7.8 Authentication failed"})
		Expect(err).NotTo(HaveOccurred())
		defer server.Close()
		allowInsecureTLSTestDial()

		sender, err := New(SMTPConfig{
			Host:        server.host,
			Port:        server.port,
			Mode:        "tls",
			SenderEmail: "mailer@example.com",
			Password:    "secret",
		}, mustLogger())
		Expect(err).NotTo(HaveOccurred())

		err = sender.Send(context.Background(), validEmail())

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("smtp auth"))
	})

	It("wraps smtp mail from failures", func() {
		server, err := startTLSSMTPServer(smtpServerBehavior{mailFromResponse: "550 5.1.0 Sender rejected"})
		Expect(err).NotTo(HaveOccurred())
		defer server.Close()
		allowInsecureTLSTestDial()

		sender, err := New(SMTPConfig{
			Host:        server.host,
			Port:        server.port,
			Mode:        "tls",
			SenderEmail: "mailer@example.com",
			Password:    "secret",
		}, mustLogger())
		Expect(err).NotTo(HaveOccurred())

		err = sender.Send(context.Background(), validEmail())

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("smtp mail from"))
	})

	It("wraps smtp recipient failures", func() {
		server, err := startTLSSMTPServer(smtpServerBehavior{rcptToResponse: "550 5.1.1 Recipient rejected"})
		Expect(err).NotTo(HaveOccurred())
		defer server.Close()
		allowInsecureTLSTestDial()

		sender, err := New(SMTPConfig{
			Host:        server.host,
			Port:        server.port,
			Mode:        "tls",
			SenderEmail: "mailer@example.com",
			Password:    "secret",
		}, mustLogger())
		Expect(err).NotTo(HaveOccurred())

		err = sender.Send(context.Background(), validEmail())

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("smtp rcpt to"))
	})

	It("wraps smtp data command failures", func() {
		server, err := startTLSSMTPServer(smtpServerBehavior{dataResponse: "554 5.5.1 DATA rejected"})
		Expect(err).NotTo(HaveOccurred())
		defer server.Close()
		allowInsecureTLSTestDial()

		sender, err := New(SMTPConfig{
			Host:        server.host,
			Port:        server.port,
			Mode:        "tls",
			SenderEmail: "mailer@example.com",
			Password:    "secret",
		}, mustLogger())
		Expect(err).NotTo(HaveOccurred())

		err = sender.Send(context.Background(), validEmail())

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("smtp data"))
	})

	It("wraps smtp write failures", func() {
		server, err := startTLSSMTPServer(smtpServerBehavior{closeOnData: true})
		Expect(err).NotTo(HaveOccurred())
		defer server.Close()
		allowInsecureTLSTestDial()

		sender, err := New(SMTPConfig{
			Host:        server.host,
			Port:        server.port,
			Mode:        "tls",
			SenderEmail: "mailer@example.com",
			Password:    "secret",
		}, mustLogger())
		Expect(err).NotTo(HaveOccurred())

		err = sender.Send(context.Background(), validEmail())

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("smtp write message"))
	})
})

var _ = Describe("error wrappers", func() {
	It("wraps smtp adapter errors", func() {
		base := errors.New("boom")

		Expect(WrapDialSMTPError(base).Error()).To(ContainSubstring("dial smtp"))
		Expect(WrapCreateSMTPClientError(base).Error()).To(ContainSubstring("create smtp client"))
		Expect(WrapStartTLSError(base).Error()).To(ContainSubstring("starttls smtp client"))
		Expect(WrapSMTPAuthError(base).Error()).To(ContainSubstring("smtp auth"))
		Expect(WrapSMTPMailFromError(base).Error()).To(ContainSubstring("smtp mail from"))
		Expect(WrapSMTPRcptToError(base).Error()).To(ContainSubstring("smtp rcpt to"))
		Expect(WrapSMTPDataError(base).Error()).To(ContainSubstring("smtp data"))
		Expect(WrapSMTPWriteError(base).Error()).To(ContainSubstring("smtp write message"))
	})
})

type tlsSMTPServer struct {
	listener net.Listener
	host     string
	port     int
	payload  chan string
	behavior smtpServerBehavior
}

func (s *tlsSMTPServer) Close() {
	_ = s.listener.Close()
}

type smtpServerBehavior struct {
	authResponse     string
	mailFromResponse string
	rcptToResponse   string
	dataResponse     string
	closeOnData      bool
}

func startTLSSMTPServer(behavior smtpServerBehavior) (*tlsSMTPServer, error) {
	cert, err := generateTLSCert()
	if err != nil {
		return nil, err
	}

	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{cert}})
	if err != nil {
		return nil, err
	}

	server := &tlsSMTPServer{
		listener: listener,
		host:     "127.0.0.1",
		port:     listener.Addr().(*net.TCPAddr).Port,
		payload:  make(chan string, 1),
		behavior: behavior,
	}

	go server.serve()
	return server, nil
}

type plainSMTPServer struct {
	listener net.Listener
	host     string
	port     int
	behavior plainSMTPBehavior
}

type plainSMTPBehavior struct {
	closeImmediately bool
	startTLSResponse string
}

func (s *plainSMTPServer) Close() {
	_ = s.listener.Close()
}

func startPlainSMTPServer(behavior plainSMTPBehavior) (*plainSMTPServer, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}

	server := &plainSMTPServer{
		listener: listener,
		host:     "127.0.0.1",
		port:     listener.Addr().(*net.TCPAddr).Port,
		behavior: behavior,
	}

	go server.serve()
	return server, nil
}

func (s *tlsSMTPServer) serve() {
	conn, err := s.listener.Accept()
	if err != nil {
		return
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)
	writeLine := func(line string) error {
		if _, err := writer.WriteString(line + "\r\n"); err != nil {
			return err
		}
		return writer.Flush()
	}

	_ = writeLine("220 localhost ESMTP ready")
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}

		switch {
		case hasPrefix(line, "EHLO"), hasPrefix(line, "HELO"):
			_ = writeLine("250-localhost")
			_ = writeLine("250 AUTH PLAIN")
		case hasPrefix(line, "AUTH"):
			_ = writeLine(defaultString(s.behavior.authResponse, "235 2.7.0 Authentication successful"))
		case hasPrefix(line, "MAIL FROM"):
			_ = writeLine(defaultString(s.behavior.mailFromResponse, "250 2.1.0 Ok"))
		case hasPrefix(line, "RCPT TO"):
			_ = writeLine(defaultString(s.behavior.rcptToResponse, "250 2.1.5 Ok"))
		case hasPrefix(line, "DATA"):
			if s.behavior.dataResponse != "" {
				_ = writeLine(s.behavior.dataResponse)
				continue
			}

			_ = writeLine("354 End data with <CR><LF>.<CR><LF>")
			if s.behavior.closeOnData {
				return
			}
			payload, readErr := readData(reader)
			if readErr != nil {
				return
			}
			s.payload <- payload
			_ = writeLine("250 2.0.0 Ok: queued")
		case hasPrefix(line, "QUIT"):
			_ = writeLine("221 2.0.0 Bye")
			return
		default:
			_ = writeLine("250 Ok")
		}
	}
}

func (s *plainSMTPServer) serve() {
	conn, err := s.listener.Accept()
	if err != nil {
		return
	}
	defer conn.Close()

	if s.behavior.closeImmediately {
		return
	}

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)
	writeLine := func(line string) error {
		if _, err := writer.WriteString(line + "\r\n"); err != nil {
			return err
		}
		return writer.Flush()
	}

	_ = writeLine("220 localhost ESMTP ready")
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}

		switch {
		case hasPrefix(line, "EHLO"), hasPrefix(line, "HELO"):
			_ = writeLine("250-localhost")
			_ = writeLine("250 STARTTLS")
		case hasPrefix(line, "STARTTLS"):
			_ = writeLine(defaultString(s.behavior.startTLSResponse, "454 TLS not available"))
			return
		default:
			_ = writeLine("250 Ok")
		}
	}
}

func readData(reader *bufio.Reader) (string, error) {
	var data string
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		if line == ".\r\n" {
			return data, nil
		}
		data += line
	}
}

func hasPrefix(line, prefix string) bool {
	return len(line) >= len(prefix) && line[:len(prefix)] == prefix
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func allowInsecureTLSTestDial() {
	prevTLSConfigFactory := newTLSConfig
	newTLSConfig = func(host string) *tls.Config {
		return &tls.Config{
			ServerName:         host,
			InsecureSkipVerify: true,
		}
	}
	DeferCleanup(func() {
		newTLSConfig = prevTLSConfigFactory
	})
}

func mustLogger() logging.Logger {
	lg, err := logging.New("mail-service", "test", "debug")
	Expect(err).NotTo(HaveOccurred())
	return lg
}

func validEmail() mail.Email {
	return mail.Email{
		To:       "user@example.com",
		Subject:  "Welcome",
		HTMLBody: "<p>Hello</p>",
	}
}

var _ = Describe("smtpSender transport bootstrap", func() {
	It("wraps smtp client creation failures after tcp dial", func() {
		server, err := startPlainSMTPServer(plainSMTPBehavior{closeImmediately: true})
		Expect(err).NotTo(HaveOccurred())
		defer server.Close()

		lg, err := logging.New("mail-service", "test", "debug")
		Expect(err).NotTo(HaveOccurred())

		sender, err := New(SMTPConfig{
			Host:        server.host,
			Port:        server.port,
			Mode:        "starttls",
			SenderEmail: "mailer@example.com",
			Password:    "secret",
		}, lg)
		Expect(err).NotTo(HaveOccurred())

		err = sender.Send(context.Background(), validEmail())

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("create smtp client"))
	})

	It("wraps starttls negotiation failures", func() {
		server, err := startPlainSMTPServer(plainSMTPBehavior{startTLSResponse: "454 TLS not available"})
		Expect(err).NotTo(HaveOccurred())
		defer server.Close()

		lg, err := logging.New("mail-service", "test", "debug")
		Expect(err).NotTo(HaveOccurred())

		sender, err := New(SMTPConfig{
			Host:        server.host,
			Port:        server.port,
			Mode:        "starttls",
			SenderEmail: "mailer@example.com",
			Password:    "secret",
		}, lg)
		Expect(err).NotTo(HaveOccurred())

		err = sender.Send(context.Background(), validEmail())

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("starttls smtp client"))
	})
})

func generateTLSCert() (tls.Certificate, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, err
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "127.0.0.1",
		},
		NotBefore: time.Now().Add(-time.Hour),
		NotAfter:  time.Now().Add(time.Hour),
		KeyUsage:  x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
		},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return tls.Certificate{}, err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})

	return tls.X509KeyPair(certPEM, keyPEM)
}
