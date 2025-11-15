# Logs Receiver

The Logs Receiver is a custom OpenTelemetry Collector receiver that polls HTTP endpoints to collect logs.

## Configuration

### Top-level Configuration

- `collection_interval` (duration): How often to poll the endpoints for logs. Default: 30s
- `targets` (array): List of target endpoints to poll for logs

### Target Configuration

Each target in the `targets` array supports:

- `endpoint` (string, required): HTTP endpoint URL to poll
- `method` (string): HTTP method to use. Default: "GET"
- `body` (string): Request body content for POST/PUT requests
- `headers` (map[string]string): HTTP headers to send with the request
- `service_name` (string): Service name to assign to logs. Default: "logs-receiver"
- `log_level` (string): Log level to assign to produced log records. Default: "info"
- `labels` (map[string]string): Extracted labels added to each log record (see below)

### Labels
if the value matches a top-level key or a dot-separated path (e.g. `qualified: "auditData.qualifiedBusinessObject"`), the receiver will extract that key/path from each JSON log object. When an array is encountered along the path, values from all elements are aggregated.
If a path cannot be resolved, the label value is set to `NOT FOUND`.

## Format Detection
The receiver inspects the `Content-Type` response header:
- Contains `application/json` -> parsed as JSON
- Contains `text` (or anything else) -> treated as plain text (each non-empty line becomes a log record)

## Example Configuration
```yaml
receivers:
  logsreceiver:
    collection_interval: 10s
    targets:
      - endpoint: "https://jsonplaceholder.typicode.com/users"  
        method: "GET"
        service_name: "scrap-users"
        log_level: "info"
        labels:
          companies: "company.name"
          user_emails: "email"

          
      - endpoint: "https://example.com/audit"
        method: "POST"
        body: '{"fromDate":"2025-10-15", "toDate":"2025-10-31"}'
        headers:
          Authorization: "Basic YWRtaW46YWRtaW4="
        labels:
          emails: "action.email"  # nested path
          status_array: "data.closing_status" # will aggregate if array encountered

exporters:
  debug:
    verbosity: detailed

service:
  pipelines:
    logs:
      receivers: [logsreceiver]
      exporters: [debug]
```

## JSON Handling
For JSON responses:
- If the top-level value is an array, each element becomes a log record.
- If the top-level value is an object, it becomes a single log record.
- Dynamic labels extract values using dot paths and aggregate over arrays.

## Text Handling
For text responses (non-JSON), each non-empty line becomes a log record with severity derived from `log_level`.

## Features
- Poll multiple HTTP endpoints on a configured interval.
- Automatic format detection via HTTP `Content-Type`.
- Dynamic label extraction with nested path and array aggregation.
- Timestamp parsing from configurable field.
- Structured JSON object fallback to full object as message body.
- Graceful error handling and endpoint attribution.

## Use Cases
- Poll REST APIs that expose operational or audit data.
- Convert structured JSON responses into OpenTelemetry logs.
- Enrich logs with dynamically extracted nested fields.
- Ingest simple text endpoints as line-based logs.
