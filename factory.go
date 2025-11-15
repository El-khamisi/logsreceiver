// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/logsreceiver"

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

const (
	// typeStr is the value of the "type" key in configuration.
	typeStr = "logsreceiver"

	// stability is the current stability level of the component.
	stability = component.StabilityLevelAlpha
)

var errConfigNotLogsReceiver = errors.New("config was not a logs receiver config")

// NewFactory creates a new logs receiver factory.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		component.MustNewType(typeStr),
		createDefaultConfig,
		receiver.WithLogs(createLogsReceiver, stability))
}

// createDefaultConfig creates the default configuration for the logs receiver.
func createDefaultConfig() component.Config {
	return &Config{
		CollectionInterval: 30 * time.Second,
		Targets:            []*targetConfig{},
	}
}

// createLogsReceiver creates a logs receiver based on provided config.
func createLogsReceiver(_ context.Context, params receiver.Settings, rConf component.Config, consumer consumer.Logs) (receiver.Logs, error) {
	cfg, ok := rConf.(*Config)
	if !ok {
		return nil, errConfigNotLogsReceiver
	}

	logsReceiver := newLogsReceiver(cfg, params, consumer)
	return logsReceiver, nil
}
