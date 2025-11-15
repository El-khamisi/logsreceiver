// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logsreceiver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				CollectionInterval: 10 * time.Second,
				Targets: []*targetConfig{
					{
						Endpoint: "http://example.com/logs",
						Method:   "GET",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "no targets",
			config: Config{
				CollectionInterval: 10 * time.Second,
				Targets:            []*targetConfig{},
			},
			wantErr: true,
		},
		{
			name: "invalid endpoint",
			config: Config{
				CollectionInterval: 10 * time.Second,
				Targets: []*targetConfig{
					{
						Endpoint: "invalid-url",
						Method:   "GET",
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestTargetConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  targetConfig
		wantErr bool
	}{
		{
			name: "valid config with defaults",
			config: targetConfig{
				Endpoint: "https://api.example.com/logs",
			},
			wantErr: false,
		},
		{
			name: "valid config with custom values",
			config: targetConfig{
				Endpoint:    "https://api.example.com/logs",
				Method:      "POST",
				ServiceName: "my-service",
				LogLevel:    "debug",
				Labels:      map[string]string{"env": "test", "region": "us"},
			},
			wantErr: false,
		},
		{
			name: "missing endpoint",
			config: targetConfig{
				Method: "GET",
			},
			wantErr: true,
		},
		{
			name: "invalid endpoint",
			config: targetConfig{
				Endpoint: "not-a-url",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				// Check defaults are set
				if tt.config.Method == "" {
					assert.Equal(t, "GET", tt.config.Method)
				}
				if tt.config.ServiceName == "" {
					assert.Equal(t, "logs-receiver", tt.config.ServiceName)
				}
				if tt.config.LogLevel == "" {
					assert.Equal(t, "info", tt.config.LogLevel)
				}
			}
		})
	}
}
