package logsreceiver

import (
	"errors"
	"fmt"
	"net/url"
	"time"
)

// Predefined error responses for configuration validation failures
var (
	errInvalidEndpoint = errors.New(`"endpoint" must be in the form of <scheme>://<hostname>[:<port>]`)
	errMissingEndpoint = errors.New("endpoint must be specified")
)

type Config struct {
	CollectionInterval time.Duration `mapstructure:"collection_interval"`

	Targets []*targetConfig `mapstructure:"targets"`

	_ struct{}
}

type targetConfig struct {
	Endpoint string `mapstructure:"endpoint"`

	Method string `mapstructure:"method"`

	Body string `mapstructure:"body"`

	Headers map[string]string `mapstructure:"headers"`

	// Service name to assign to logs
	ServiceName string `mapstructure:"service_name"`

	// Log level to assign to logs
	LogLevel string `mapstructure:"log_level"`

	// Additional attributes to add to each log record
	Labels map[string]string `mapstructure:"labels"`
}

func (cfg *targetConfig) Validate() error {
	if cfg.Endpoint == "" {
		return errMissingEndpoint
	}

	if _, parseErr := url.ParseRequestURI(cfg.Endpoint); parseErr != nil {
		return fmt.Errorf("%s: %w", errInvalidEndpoint.Error(), parseErr)
	}

	if cfg.Method == "" {
		cfg.Method = "GET"
	}

	if cfg.ServiceName == "" {
		cfg.ServiceName = "logs-receiver"
	}

	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}

	return nil
}

func (cfg *Config) Validate() error {
	if len(cfg.Targets) == 0 {
		return errors.New("no targets configured")
	}

	if cfg.CollectionInterval <= 0 {
		cfg.CollectionInterval = 30 * time.Second
	}

	for _, target := range cfg.Targets {
		if err := target.Validate(); err != nil {
			return err
		}
	}

	return nil
}
