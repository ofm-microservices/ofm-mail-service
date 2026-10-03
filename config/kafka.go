package config

// KafkaConfig defines the Kafka broker and consumer group used by mail-service.
type KafkaConfig struct {
	Brokers []string `env:"KAFKA_BROKERS" envSeparator:"," envDefault:"127.0.0.1:9092"`
	GroupID string   `env:"KAFKA_MAIL_GROUP_ID" envDefault:"mail-service"`
	CommandTopic string `env:"KAFKA_MAIL_COMMAND_TOPIC" envDefault:"mail.send"`
	ResultTopic string `env:"KAFKA_MAIL_RESULT_TOPIC" envDefault:"mail.send.result"`
}
