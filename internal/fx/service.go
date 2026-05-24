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
		fx.Annotate(ProvideOrderDeliveryTemplateDefinition, fx.ResultTags(`group:"mail_templates"`)),
		fx.Annotate(ProvideOrderDeliveryBuyerTemplateDefinition, fx.ResultTags(`group:"mail_templates"`)),
		fx.Annotate(ProvideOrderRevisionTemplateDefinition, fx.ResultTags(`group:"mail_templates"`)),
		fx.Annotate(ProvideOrderRevisionBuyerTemplateDefinition, fx.ResultTags(`group:"mail_templates"`)),
		fx.Annotate(ProvideOrderDisputeTemplateDefinition, fx.ResultTags(`group:"mail_templates"`)),
		fx.Annotate(ProvideOrderDisputeSellerTemplateDefinition, fx.ResultTags(`group:"mail_templates"`)),
		fx.Annotate(ProvideOrderCompletionTemplateDefinition, fx.ResultTags(`group:"mail_templates"`)),
		fx.Annotate(ProvideOrderCompletionSellerTemplateDefinition, fx.ResultTags(`group:"mail_templates"`)),
		fx.Annotate(ProvideOrderReleaseFailedTemplateDefinition, fx.ResultTags(`group:"mail_templates"`)),
		fx.Annotate(ProvideOrderReleaseFailedSellerTemplateDefinition, fx.ResultTags(`group:"mail_templates"`)),
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

// ProvideOrderDeliveryTemplateDefinition registers the seller delivery template definition.
func ProvideOrderDeliveryTemplateDefinition() app.TemplateDefinition {
	return app.TemplateDefinition{
		MessageType:      "order_delivery_submitted_seller",
		SubjectTemplate:  "Your order delivery was submitted for {{.gig_title}}",
		HTMLTemplatePath: "order_lifecycle.html",
	}
}

// ProvideOrderDeliveryBuyerTemplateDefinition registers the buyer delivery acknowledgement template.
func ProvideOrderDeliveryBuyerTemplateDefinition() app.TemplateDefinition {
	return app.TemplateDefinition{
		MessageType:      "order_delivery_submitted_buyer",
		SubjectTemplate:  "A delivery was submitted for your order {{.order_id}}",
		HTMLTemplatePath: "order_lifecycle.html",
	}
}

// ProvideOrderRevisionTemplateDefinition registers the seller revision template definition.
func ProvideOrderRevisionTemplateDefinition() app.TemplateDefinition {
	return app.TemplateDefinition{
		MessageType:      "order_revision_requested_seller",
		SubjectTemplate:  "Revision requested for {{.gig_title}}",
		HTMLTemplatePath: "order_lifecycle.html",
	}
}

// ProvideOrderRevisionBuyerTemplateDefinition registers the buyer revision confirmation template.
func ProvideOrderRevisionBuyerTemplateDefinition() app.TemplateDefinition {
	return app.TemplateDefinition{
		MessageType:      "order_revision_requested_buyer",
		SubjectTemplate:  "Your revision request for order {{.order_id}} was received",
		HTMLTemplatePath: "order_lifecycle.html",
	}
}

// ProvideOrderDisputeTemplateDefinition registers the dispute template definition.
func ProvideOrderDisputeTemplateDefinition() app.TemplateDefinition {
	return app.TemplateDefinition{
		MessageType:      "order_disputed_buyer",
		SubjectTemplate:  "A dispute was opened for your order {{.order_id}}",
		HTMLTemplatePath: "order_lifecycle.html",
	}
}

// ProvideOrderDisputeSellerTemplateDefinition registers the seller dispute template.
func ProvideOrderDisputeSellerTemplateDefinition() app.TemplateDefinition {
	return app.TemplateDefinition{
		MessageType:      "order_disputed_seller",
		SubjectTemplate:  "A dispute was opened for order {{.order_id}}",
		HTMLTemplatePath: "order_lifecycle.html",
	}
}

// ProvideOrderCompletionTemplateDefinition registers the completion template definition.
func ProvideOrderCompletionTemplateDefinition() app.TemplateDefinition {
	return app.TemplateDefinition{
		MessageType:      "order_completed_buyer",
		SubjectTemplate:  "Your order {{.order_id}} is complete",
		HTMLTemplatePath: "order_lifecycle.html",
	}
}

// ProvideOrderCompletionSellerTemplateDefinition registers the seller completion template.
func ProvideOrderCompletionSellerTemplateDefinition() app.TemplateDefinition {
	return app.TemplateDefinition{
		MessageType:      "order_completed_seller",
		SubjectTemplate:  "Your order {{.order_id}} has been completed",
		HTMLTemplatePath: "order_lifecycle.html",
	}
}

// ProvideOrderReleaseFailedTemplateDefinition registers the payout failure template definition.
func ProvideOrderReleaseFailedTemplateDefinition() app.TemplateDefinition {
	return app.TemplateDefinition{
		MessageType:      "order_release_failed_buyer",
		SubjectTemplate:  "We could not release funds for order {{.order_id}}",
		HTMLTemplatePath: "order_lifecycle.html",
	}
}

// ProvideOrderReleaseFailedSellerTemplateDefinition registers the seller release failure template.
func ProvideOrderReleaseFailedSellerTemplateDefinition() app.TemplateDefinition {
	return app.TemplateDefinition{
		MessageType:      "order_release_failed_seller",
		SubjectTemplate:  "Payout release failed for order {{.order_id}}",
		HTMLTemplatePath: "order_lifecycle.html",
	}
}
