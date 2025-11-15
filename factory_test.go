// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logsreceiver

import (
	"context"
	"testing"

	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestFactory_CreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	if cfg == nil {
		t.Fatal("CreateDefaultConfig() returned nil")
	}

	logsConfig, ok := cfg.(*Config)
	if !ok {
		t.Fatal("CreateDefaultConfig() did not return *Config")
	}

	if logsConfig.CollectionInterval <= 0 {
		t.Error("Default collection interval should be positive")
	}

	if logsConfig.Targets == nil {
		t.Error("Targets should be initialized")
	}
}

func TestFactory_CreateLogsReceiver(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	// Add a valid target for testing
	logsConfig := cfg.(*Config)
	logsConfig.Targets = []*targetConfig{
		{
			Endpoint: "http://example.com/logs",
			Method:   "GET",
		},
	}

	ctx := context.Background()
	settings := receivertest.NewNopSettings(factory.Type())
	consumer := consumertest.NewNop()

	receiver, err := factory.CreateLogs(ctx, settings, cfg, consumer)
	if err != nil {
		t.Fatalf("CreateLogs() failed: %v", err)
	}

	if receiver == nil {
		t.Fatal("CreateLogs() returned nil receiver")
	}
}
