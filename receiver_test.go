package logsreceiver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

// testLogsSink is a simple logs consumer used for tests.
type testLogsSink struct {
	mu   sync.Mutex
	logs []plog.Logs
}

func (s *testLogsSink) ConsumeLogs(_ context.Context, ld plog.Logs) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logs = append(s.logs, ld)
	return nil
}

func (s *testLogsSink) AllLogs() []plog.Logs {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]plog.Logs(nil), s.logs...)
}

func (s *testLogsSink) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// waitForLogs polls the sink until at least minRecords or timeout.
func waitForLogs(t *testing.T, sink *testLogsSink, minRecords int, timeout time.Duration) plog.Logs {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		logs := sink.AllLogs()
		count := 0
		for _, l := range logs {
			count += l.LogRecordCount()
		}
		if count >= minRecords && len(logs) > 0 {
			return logs[len(logs)-1]
		}
		time.Sleep(10 * time.Millisecond)
	}
	return plog.NewLogs()
}

func TestLogsReceiver_CommentsEmails(t *testing.T) {
	// Array response now yields ONE log record whose body is a slice of objects
	commentsJSON := `[{"postId":1,"id":1,"name":"id labore ex et quam laborum","email":"Eliseo@gardner.biz"},{"postId":1,"id":2,"name":"quo vero reiciendis velit similique earum","email":"Jayne_Kuhic@sydney.com"}]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(commentsJSON))
	}))
	defer srv.Close()

	cfg := &Config{CollectionInterval: 20 * time.Millisecond, Targets: []*targetConfig{{Endpoint: srv.URL, Method: "GET", LogLevel: "debug", ServiceName: "scrap-comments", Labels: map[string]string{"user_emails": "email"}}}}
	settings := receivertest.NewNopSettings(component.MustNewType("logsreceiver"))
	sink := &testLogsSink{}
	r := newLogsReceiver(cfg, settings, sink)

	ctx := context.Background()
	if err := r.Start(ctx, nil); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	logs := waitForLogs(t, sink, 1, 600*time.Millisecond)
	if logs.LogRecordCount() != 1 {
		_ = r.Shutdown(ctx)
		t.Fatalf("expected 1 log record for array top-level, got %d", logs.LogRecordCount())
	}
	lr := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	val, ok := lr.Attributes().Get("user_emails")
	if !ok {
		_ = r.Shutdown(ctx)
		t.Fatalf("expected user_emails attribute")
	}
	attrStr := val.Str()
	if !strings.Contains(attrStr, "Eliseo@gardner.biz") || !strings.Contains(attrStr, "Jayne_Kuhic@sydney.com") {
		_ = r.Shutdown(ctx)
		t.Errorf("expected aggregated emails, got %s", attrStr)
	}
	// verify structured body slice length
	if lr.Body().Type() != pcommon.ValueTypeSlice {
		t.Errorf("expected slice body, got %v", lr.Body().Type())
	}
	if lr.Body().Slice().Len() != 2 {
		t.Errorf("expected body slice len 2 got %d", lr.Body().Slice().Len())
	}
	if err := r.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}
}

func TestLogsReceiver_UsersCompanies(t *testing.T) {
	usersJSON := `[{"id":1,"name":"Leanne Graham","company":{"name":"Romaguera-Crona"}},{"id":2,"name":"Ervin Howell","company":{"name":"Deckow-Crist"}}]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(usersJSON))
	}))
	defer srv.Close()

	cfg := &Config{CollectionInterval: 25 * time.Millisecond, Targets: []*targetConfig{{Endpoint: srv.URL, Method: "GET", LogLevel: "info", Labels: map[string]string{"companies": "company.name"}}}}
	settings := receivertest.NewNopSettings(component.MustNewType("logsreceiver"))
	sink := &testLogsSink{}
	r := newLogsReceiver(cfg, settings, sink)

	ctx := context.Background()
	if err := r.Start(ctx, nil); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	logs := waitForLogs(t, sink, 1, 600*time.Millisecond)
	if logs.LogRecordCount() != 1 {
		_ = r.Shutdown(ctx)
		t.Fatalf("expected 1 log record for array, got %d", logs.LogRecordCount())
	}
	lr := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	val, ok := lr.Attributes().Get("companies")
	if !ok {
		_ = r.Shutdown(ctx)
		t.Fatalf("expected companies attribute")
	}
	attrStr := val.Str()
	if !strings.Contains(attrStr, "Romaguera-Crona") || !strings.Contains(attrStr, "Deckow-Crist") {
		_ = r.Shutdown(ctx)
		t.Errorf("expected aggregated company names, got %s", attrStr)
	}
	if lr.Body().Type() != pcommon.ValueTypeSlice {
		t.Errorf("expected slice body for users array")
	}
	if lr.Body().Slice().Len() != 2 {
		t.Errorf("expected body slice len 2 got %d", lr.Body().Slice().Len())
	}
	if err := r.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}
}

func TestLogsReceiver_JSONObject_StructuredBodyAndMissingLabel(t *testing.T) {
	jsonObj := `{"id":10,"name":"example","nested":{"inner":42}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(jsonObj))
	}))
	defer srv.Close()

	cfg := &Config{CollectionInterval: 10 * time.Millisecond, Targets: []*targetConfig{{Endpoint: srv.URL, Method: "GET", LogLevel: "info", Labels: map[string]string{"id_field": "id", "missing_field": "does.not.exist"}}}}
	settings := receivertest.NewNopSettings(component.MustNewType("logsreceiver"))
	sink := &testLogsSink{}
	r := newLogsReceiver(cfg, settings, sink)
	ctx := context.Background()
	if err := r.Start(ctx, nil); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	logs := waitForLogs(t, sink, 1, 400*time.Millisecond)
	if logs.LogRecordCount() != 1 {
		_ = r.Shutdown(ctx)
		t.Fatalf("expected 1 log record got %d", logs.LogRecordCount())
	}
	lr := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	if lr.Body().Type() != pcommon.ValueTypeMap {
		t.Errorf("expected map body got %v", lr.Body().Type())
	}
	if _, ok := lr.Attributes().Get("id_field"); !ok {
		t.Errorf("expected id_field attribute")
	}
	miss, ok := lr.Attributes().Get("missing_field")
	if !ok || miss.Str() != "NOT FOUND" {
		t.Errorf("expected missing_field attribute value NOT FOUND got %v (present=%v)", miss, ok)
	}
	if err := r.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}
}

func TestLogsReceiver_TextLines(t *testing.T) {
	textResp := "alpha\nbeta\n\n gamma "
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(textResp))
	}))
	defer srv.Close()

	cfg := &Config{CollectionInterval: 100 * time.Millisecond, Targets: []*targetConfig{{Endpoint: srv.URL, Method: "GET", LogLevel: "info"}}}
	settings := receivertest.NewNopSettings(component.MustNewType("logsreceiver"))
	sink := &testLogsSink{}
	r := newLogsReceiver(cfg, settings, sink)

	if err := r.pollTarget(context.Background(), cfg.Targets[0]); err != nil {
		t.Fatalf("pollTarget failed: %v", err)
	}
	logs := sink.AllLogs()
	if len(logs) == 0 {
		t.Fatalf("expected logs")
	}
	lrCount := 0
	for _, l := range logs {
		lrCount += l.LogRecordCount()
	}
	if lrCount != 3 { // alpha, beta, gamma (trimmed) -> 3 records
		t.Errorf("expected 3 text log records got %d", lrCount)
	}
}

func TestLogsReceiver_PollTarget_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	settings := receivertest.NewNopSettings(component.MustNewType("logsreceiver"))
	r := newLogsReceiver(&Config{}, settings, &testLogsSink{})
	target := &targetConfig{Endpoint: srv.URL, Method: "GET"}
	if err := r.pollTarget(context.Background(), target); err == nil {
		t.Fatalf("expected error for 500 response")
	}
}

func TestLogsReceiver_PollTarget_InvalidJSON(t *testing.T) {
	badJSON := "{" // invalid
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(badJSON))
	}))
	defer srv.Close()

	settings := receivertest.NewNopSettings(component.MustNewType("logsreceiver"))
	r := newLogsReceiver(&Config{}, settings, &testLogsSink{})
	target := &targetConfig{Endpoint: srv.URL, Method: "GET"}
	if err := r.pollTarget(context.Background(), target); err == nil {
		t.Fatalf("expected JSON unmarshal error")
	}
}

func TestLogsReceiver_ParseLogs_UnknownContentType(t *testing.T) {
	body := "line1\nline2"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	settings := receivertest.NewNopSettings(component.MustNewType("logsreceiver"))
	r := newLogsReceiver(&Config{}, settings, &testLogsSink{})
	target := &targetConfig{Endpoint: srv.URL, Method: "GET"}
	if err := r.pollTarget(context.Background(), target); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLogsReceiver_CreateRequest_DefaultContentType_JSON(t *testing.T) {
	settings := receivertest.NewNopSettings(component.MustNewType("logsreceiver"))
	r := newLogsReceiver(&Config{}, settings, &testLogsSink{})
	target := &targetConfig{Endpoint: "http://example.com", Method: "POST", Body: "{\"k\":\"v\"}"}
	req, err := r.createRequest(context.Background(), target)
	if err != nil {
		t.Fatalf("createRequest failed: %v", err)
	}
	if ct := req.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected application/json got %s", ct)
	}
}

func TestLogsReceiver_CreateRequest_DefaultContentType_Text(t *testing.T) {
	settings := receivertest.NewNopSettings(component.MustNewType("logsreceiver"))
	r := newLogsReceiver(&Config{}, settings, &testLogsSink{})
	target := &targetConfig{Endpoint: "http://example.com", Method: "POST", Body: "plain text"}
	req, err := r.createRequest(context.Background(), target)
	if err != nil {
		t.Fatalf("createRequest failed: %v", err)
	}
	if ct := req.Header.Get("Content-Type"); ct != "text/plain" {
		t.Errorf("expected text/plain got %s", ct)
	}
}

func TestLogsReceiver_ExtractValueByPath_NestedArrays(t *testing.T) {
	entry := map[string]interface{}{
		"outer": []interface{}{
			map[string]interface{}{"inner": []interface{}{map[string]interface{}{"value": "a"}, map[string]interface{}{"value": "b"}}},
			map[string]interface{}{"inner": []interface{}{map[string]interface{}{"value": "c"}}},
		},
	}
	settings := receivertest.NewNopSettings(component.MustNewType("logsreceiver"))
	r := newLogsReceiver(&Config{}, settings, &testLogsSink{})
	res := r.extractValueByPath("outer.inner.value", entry)
	vals, ok := res.([]interface{})
	if !ok {
		t.Fatalf("expected []interface{} result")
	}
	if len(vals) != 3 {
		t.Fatalf("expected 3 aggregated values got %d", len(vals))
	}
}

func TestLogsReceiver_GetSeverityNumber(t *testing.T) {
	settings := receivertest.NewNopSettings(component.MustNewType("logsreceiver"))
	r := newLogsReceiver(&Config{}, settings, &testLogsSink{})
	cases := map[string]plog.SeverityNumber{
		"trace":   plog.SeverityNumberTrace,
		"debug":   plog.SeverityNumberDebug,
		"info":    plog.SeverityNumberInfo,
		"warn":    plog.SeverityNumberWarn,
		"warning": plog.SeverityNumberWarn,
		"error":   plog.SeverityNumberError,
		"fatal":   plog.SeverityNumberFatal,
		"other":   plog.SeverityNumberInfo,
	}
	for lvl, expected := range cases {
		if got := r.getSeverityNumber(lvl); got != expected {
			t.Errorf("severity mapping mismatch for %s: got %v want %v", lvl, got, expected)
		}
	}
}
