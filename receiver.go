// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/logsreceiver"

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

// logsReceiver implements the logs receiver interface.
type logsReceiver struct {
	config   *Config
	settings receiver.Settings
	consumer consumer.Logs
	logger   *zap.Logger
	cancel   context.CancelFunc
	done     chan struct{}
}

// newLogsReceiver creates a new logs receiver.
func newLogsReceiver(config *Config, settings receiver.Settings, consumer consumer.Logs) *logsReceiver {
	return &logsReceiver{
		config:   config,
		settings: settings,
		consumer: consumer,
		logger:   settings.Logger,
		done:     make(chan struct{}),
	}
}

// Start starts the logs receiver.
func (r *logsReceiver) Start(ctx context.Context, _ component.Host) error {
	ctx, r.cancel = context.WithCancel(ctx)

	go r.poll(ctx)

	r.logger.Info("Logs receiver started",
		zap.Duration("collection_interval", r.config.CollectionInterval),
		zap.Int("targets", len(r.config.Targets)))

	return nil
}

// Shutdown stops the logs receiver.
func (r *logsReceiver) Shutdown(_ context.Context) error {
	if r.cancel != nil {
		r.cancel()
	}

	<-r.done
	r.logger.Info("Logs receiver stopped")
	return nil
}

// poll continuously polls the configured endpoints for logs.
func (r *logsReceiver) poll(ctx context.Context) {
	defer close(r.done)

	ticker := time.NewTicker(r.config.CollectionInterval)
	defer ticker.Stop()

	// Poll immediately on start
	r.pollTargets(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.pollTargets(ctx)
		}
	}
}

// pollTargets polls all configured targets.
func (r *logsReceiver) pollTargets(ctx context.Context) {
	var wg sync.WaitGroup

	for _, target := range r.config.Targets {
		wg.Add(1)
		go func(target *targetConfig) {
			defer wg.Done()
			if err := r.pollTarget(ctx, target); err != nil {
				r.logger.Error("Failed to poll target",
					zap.String("endpoint", target.Endpoint),
					zap.Error(err))
			}
		}(target)
	}

	wg.Wait()
}

// pollTarget polls a single target endpoint.
func (r *logsReceiver) pollTarget(ctx context.Context, target *targetConfig) error {
	req, err := r.createRequest(ctx, target)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP error: %d %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	logs, err := r.parseLogs(resp, body, target)
	if err != nil {
		return fmt.Errorf("failed to parse logs: %w", err)
	}

	if logs.LogRecordCount() > 0 {
		if err := r.consumer.ConsumeLogs(ctx, logs); err != nil {
			return fmt.Errorf("failed to consume logs: %w", err)
		}

		r.logger.Debug("Successfully consumed logs",
			zap.String("endpoint", target.Endpoint),
			zap.Int("log_count", logs.LogRecordCount()))
	}

	return nil
}

// createRequest creates an HTTP request for the target.
func (r *logsReceiver) createRequest(ctx context.Context, target *targetConfig) (*http.Request, error) {
	var body io.Reader
	if target.Body != "" {
		body = strings.NewReader(target.Body)
	}

	req, err := http.NewRequestWithContext(ctx, target.Method, target.Endpoint, body)
	if err != nil {
		return nil, err
	}

	// Set headers
	for key, value := range target.Headers {
		req.Header.Set(key, value)
	}

	// Set default Content-Type for POST/PUT with body
	if target.Body != "" && req.Header.Get("Content-Type") == "" {
		if strings.HasPrefix(strings.TrimSpace(target.Body), "{") {
			req.Header.Set("Content-Type", "application/json")
		} else {
			req.Header.Set("Content-Type", "text/plain")
		}
	}

	return req, nil
}

// parseLogs parses the response body into log records.
func (r *logsReceiver) parseLogs(resp *http.Response, body []byte, target *targetConfig) (plog.Logs, error) {
	logs := plog.NewLogs()

	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	switch {
	case strings.Contains(ct, "application/json"):
		return r.parseJSONLogs(body, target, logs)
	case strings.Contains(ct, "text"):
		return r.parseTextLogs(body, target, logs)
	default:
		return r.parseTextLogs(body, target, logs)
	}
}

// parseJSONLogs parses JSON formatted logs.
func (r *logsReceiver) parseJSONLogs(body []byte, target *targetConfig, logs plog.Logs) (plog.Logs, error) {
	var jsonData interface{}
	if err := json.Unmarshal(body, &jsonData); err != nil {
		return plog.Logs{}, fmt.Errorf("failed to unmarshal JSON: %w", err)

	}

	resourceLogs := logs.ResourceLogs().AppendEmpty()
	resource := resourceLogs.Resource()

	resource.Attributes().PutStr("endpoint", target.Endpoint)
	resource.Attributes().PutStr("service.name", target.ServiceName)

	scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()

	r.addLogRecord(scopeLogs, jsonData, target)

	return logs, nil
}

// parseTextLogs parses text formatted logs.
func (r *logsReceiver) parseTextLogs(body []byte, target *targetConfig, logs plog.Logs) (plog.Logs, error) {
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	resource := resourceLogs.Resource()

	// Set resource attributes
	resource.Attributes().PutStr("service.name", target.ServiceName)
	resource.Attributes().PutStr("endpoint", target.Endpoint)

	// Add custom attributes
	for key, value := range target.Labels {
		resource.Attributes().PutStr(key, value)
	}

	scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()

	// Split text into lines and create log records
	lines := strings.Split(string(body), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		logRecord := scopeLogs.LogRecords().AppendEmpty()
		logRecord.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		logRecord.Body().SetStr(line)
		logRecord.SetSeverityText(strings.ToUpper(target.LogLevel))
		logRecord.SetSeverityNumber(r.getSeverityNumber(target.LogLevel))
	}

	return logs, nil
}

// addLogRecord adds a single log record to the scope logs.
func (r *logsReceiver) addLogRecord(scopeLogs plog.ScopeLogs, data interface{}, target *targetConfig) {
	logRecord := scopeLogs.LogRecords().AppendEmpty()
	logRecord.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))

	logRecord.SetSeverityText(strings.ToUpper(target.LogLevel))
	logRecord.SetSeverityNumber(r.getSeverityNumber(target.LogLevel))

	for key, labelVal := range target.Labels {
		if val := r.extractValueByPath(labelVal, data); val != nil {
			logRecord.Attributes().PutStr(key, fmt.Sprintf("%v", val))
		} else {
			logRecord.Attributes().PutStr(key, "NOT FOUND")
		}
	}

	r.setBodyValue(logRecord.Body(), data)
}

// setBodyValue recursively populates a pcommon.Value from an interface{} decoded from JSON.
func (r *logsReceiver) setBodyValue(dest pcommon.Value, v interface{}) {
	switch val := v.(type) {
	case map[string]interface{}:
		m := dest.SetEmptyMap()
		for k, child := range val {
			cv := m.PutEmpty(k)
			r.setBodyValue(cv, child)
		}
	case []interface{}:
		s := dest.SetEmptySlice()
		for _, elem := range val {
			cv := s.AppendEmpty()
			r.setBodyValue(cv, elem)
		}
	case string:
		dest.SetStr(val)
	case float64:
		dest.SetDouble(val)
	case bool:
		dest.SetBool(val)
	case nil:
		// Represent null as empty map to keep type (alternatively could use a string "null").
		dest.SetStr("null")
	default:
		dest.SetStr(fmt.Sprintf("%v", val))
	}
}

func (r *logsReceiver) extractValueByPath(path string, raw interface{}) interface{} {
	if raw == nil || path == "" {
		return nil
	}

	parts := strings.SplitN(path, ".", 2)
	head := parts[0]
	tail := ""
	if len(parts) == 2 {
		tail = parts[1]
	}

	switch node := raw.(type) {

	case map[string]interface{}:
		if child, ok := node[head]; ok {
			if tail == "" {
				return child
			}

			return r.extractValueByPath(tail, child)
		}
		return nil

	case []interface{}:
		var results []interface{}
		for _, el := range node {
			v := r.extractValueByPath(path, el)
			switch vv := v.(type) {
			case nil:
				// skip
			case []interface{}:
				results = append(results, vv...)
			default:
				results = append(results, vv)
			}
		}
		if len(results) == 0 {
			return nil
		}
		return results

	default:
		return nil
	}
}

// getSeverityNumber converts log level string to severity number.
func (r *logsReceiver) getSeverityNumber(level string) plog.SeverityNumber {
	switch strings.ToLower(level) {
	case "trace":
		return plog.SeverityNumberTrace
	case "debug":
		return plog.SeverityNumberDebug
	case "info":
		return plog.SeverityNumberInfo
	case "warn", "warning":
		return plog.SeverityNumberWarn
	case "error":
		return plog.SeverityNumberError
	case "fatal":
		return plog.SeverityNumberFatal
	default:
		return plog.SeverityNumberInfo
	}
}
