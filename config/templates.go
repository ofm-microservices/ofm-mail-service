package config

// TemplatesConfig defines the template asset paths used by mail-service.
type TemplatesConfig struct {
	Dir string `env:"MAIL_TEMPLATE_DIR" envDefault:"templates"`
}
