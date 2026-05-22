package appfx

import (
	"path/filepath"

	"github.com/ofm-microservices/ofm-common/pkg/logging"
	"mail-service/config"
	app "mail-service/internal/application"
	smtpmailer "mail-service/pkg/mailer/smtp"

	"go.uber.org/fx"
)

// ServiceModule wires mail delivery and template services into the FX graph.
var ServiceModule = fx.Options(
	fx.Provide(
		ProvideMailSender,
		ProvideTemplateRegistry,
		ProvideMailService,
		fx.Annotate(ProvideEmailCodeTemplateDefinition, fx.ResultTags(`group:"mail_templates"`)),
		fx.Annotate(ProvideWelcomeTemplateDefinition, fx.ResultTags(`group:"mail_templates"`)),
		fx.Annotate(ProvidePasswordResetTemplateDefinition, fx.ResultTags(`group:"mail_templates"`)),
		fx.Annotate(ProvideOrderReceiptTemplateDefinition, fx.ResultTags(`group:"mail_templates"`)),
	),
)

// ProvideMailSender constructs the SMTP-backed mail sender.
func ProvideMailSender(cfg *config.Config, lg logging.Logger) (app.MailSender, error) {
	return smtpmailer.New(cfg.SMTP, lg)
}

type templateRegistryParams struct {
	fx.In

	Cfg  *config.Config
	Defs []app.TemplateDefinition `group:"mail_templates"`
}

// ProvideTemplateRegistry resolves configured template paths and builds the
// template registry used by mail-service.
func ProvideTemplateRegistry(params templateRegistryParams) (app.TemplateRegistry, error) {
	resolvedDefs := make([]app.TemplateDefinition, 0, len(params.Defs))
	for _, def := range params.Defs {
		resolvedDefs = append(resolvedDefs, app.TemplateDefinition{
			MessageType:      def.MessageType,
			SubjectTemplate:  def.SubjectTemplate,
			HTMLTemplatePath: filepath.Join(params.Cfg.Templates.Dir, filepath.Base(def.HTMLTemplatePath)),
		})
	}

	registry, err := app.BuildTemplateRegistry(resolvedDefs)
	if err != nil {
		return nil, app.WrapTemplateDefinitionGroupError(err)
	}
	return registry, nil
}

// ProvideMailService constructs the application service responsible for
// rendering and sending email messages.
func ProvideMailService(sender app.MailSender, registry app.TemplateRegistry, lg logging.Logger) (app.MailService, error) {
	return app.New(sender, registry, lg)
}

// ProvideEmailCodeTemplateDefinition registers the email verification template
// definition.
func ProvideEmailCodeTemplateDefinition() app.TemplateDefinition {
	return app.TemplateDefinition{
		MessageType:      "email_code",
		SubjectTemplate:  "Your OFM verification code",
		HTMLTemplatePath: "email_code.html",
	}
}

// ProvideWelcomeTemplateDefinition registers the welcome message template
// definition.
func ProvideWelcomeTemplateDefinition() app.TemplateDefinition {
	return app.TemplateDefinition{
		MessageType:      "welcome",
		SubjectTemplate:  "Welcome to OFM, {{.name}}",
		HTMLTemplatePath: "welcome.html",
	}
}

// ProvidePasswordResetTemplateDefinition registers the password reset template
// definition.
func ProvidePasswordResetTemplateDefinition() app.TemplateDefinition {
	return app.TemplateDefinition{
		MessageType:      "password_reset",
		SubjectTemplate:  "Your OFM password reset code",
		HTMLTemplatePath: "password_reset.html",
	}
}

// ProvideOrderReceiptTemplateDefinition registers the order receipt template
// definition.
func ProvideOrderReceiptTemplateDefinition() app.TemplateDefinition {
	return app.TemplateDefinition{
		MessageType:      "order_receipt",
		SubjectTemplate:  "Your OFM order receipt for {{.gig_title}}",
		HTMLTemplatePath: "order_receipt.html",
	}
}
