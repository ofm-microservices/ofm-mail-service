package nats

import "mail-service/config"

type pullConsumerConfigValidator struct{}

func newPullConsumerConfigValidator() PullConsumerConfigValidator {
	return &pullConsumerConfigValidator{}
}

func (v *pullConsumerConfigValidator) Validate(cfg config.PullConsumerConfig) error {
	if cfg.Stream == "" {
		return ErrEmptyStreamName
	}
	if cfg.Subject == "" {
		return ErrEmptySubject
	}
	if cfg.Durable == "" {
		return ErrEmptyDurableName
	}
	if cfg.BatchSize <= 0 {
		return ErrInvalidBatchSize
	}
	if cfg.MaxWait <= 0 {
		return ErrInvalidMaxWait
	}
	if cfg.Workers <= 0 {
		return ErrInvalidWorkerCount
	}
	if cfg.QueueSize <= 0 {
		return ErrInvalidQueueSize
	}
	if cfg.AckWait <= 0 {
		return ErrInvalidAckWait
	}
	if cfg.MaxDeliver <= 0 {
		return ErrInvalidMaxDeliver
	}
	if !cfg.Adaptive.Enabled {
		return nil
	}
	if cfg.Adaptive.CheckInterval <= 0 {
		return ErrInvalidAdaptiveCheckInterval
	}
	if cfg.Adaptive.MediumPending < 0 || cfg.Adaptive.HighPending <= cfg.Adaptive.MediumPending {
		return ErrInvalidAdaptiveThresholds
	}
	if cfg.Adaptive.LowBatchSize <= 0 || cfg.Adaptive.LowMaxWait <= 0 ||
		cfg.Adaptive.MediumBatchSize <= 0 || cfg.Adaptive.MediumMaxWait <= 0 ||
		cfg.Adaptive.HighBatchSize <= 0 || cfg.Adaptive.HighMaxWait <= 0 {
		return ErrInvalidAdaptivePlan
	}

	return nil
}
