// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package logsreceiver implements a custom OpenTelemetry Collector receiver
// that polls HTTP endpoints to collect logs.
//
// This receiver is designed to pull logs from HTTP APIs on a configurable interval,
// parse the response in various formats (JSON, text), and convert them into
// OpenTelemetry log data for further processing in the collector pipeline.
package logsreceiver
