package tracing

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

func TestInitNoopTracer(t *testing.T) {
	p, err := Init(Config{Enabled: false, ServiceName: "test"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, span := StartSpan(context.Background(), p.Tracer(), "test", attribute.String("authorization", "TEST_ACCESS_TOKEN_UNIQUE"))
	EndSpan(span, nil, "")
	if span == nil {
		t.Fatal("expected span")
	}
	_ = ctx
}

func TestParseTraceParentInvalid(t *testing.T) {
	if _, _, err := ParseTraceParent("invalid"); err == nil {
		t.Fatal("expected error")
	}
}

func TestSanitizeAttrsDropsSecrets(t *testing.T) {
	attrs := sanitizeAttrs([]attribute.KeyValue{
		attribute.String("app.module", "webhook"),
		attribute.String("access_token", "TEST_ACCESS_TOKEN_UNIQUE"),
	})
	for _, a := range attrs {
		if strings.Contains(string(a.Key), "token") {
			t.Fatal("token attr should be dropped")
		}
	}
}

func TestHTTPExporterSendsStandardOTLPToMockCollector(t *testing.T) {
	var received atomic.Int64
	requests := make(chan otlpTraceExportRequest, 1)
	bodies := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method %s", r.Method)
		}
		if r.URL.Path != "/v1/traces" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if got := r.Header.Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
			t.Fatalf("unexpected content-type %s", got)
		}
		var req otlpTraceExportRequest
		var raw strings.Builder
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&req); err != nil {
			t.Fatalf("invalid otlp json: %v", err)
		}
		data, _ := json.Marshal(req)
		raw.Write(data)
		bodies <- raw.String()
		requests <- req
		received.Add(1)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()
	var exported atomic.Int64
	p, err := Init(Config{
		Enabled:       true,
		ServiceName:   "test",
		Version:       "1.2.3",
		Environment:   "test",
		SampleRatio:   1,
		OTLPEndpoint:  srv.URL,
		OTLPProtocol:  "http/json",
		ExportTimeout: time.Second,
		QueueSize:     16,
		BatchSize:     8,
		OnExportOK: func(n int) {
			exported.Add(int64(n))
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, root := p.Tracer().Start(context.Background(), "root-export",
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String("app.module", "tracing"),
			attribute.Int64("item.count", 3),
			attribute.Bool("cache.hit", true),
			attribute.String("access_token", "TEST_ACCESS_TOKEN_UNIQUE"),
			attribute.String("signed_url", "TEST_SIGNED_URL_UNIQUE"),
		),
	)
	_, child := p.Tracer().Start(ctx, "child-export",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attribute.String("http.method", "GET")),
	)
	EndSpan(child, errors.New("timeout"), "timeout")
	EndSpan(root, nil, "")
	shCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := p.Shutdown(shCtx); err != nil {
		t.Fatal(err)
	}
	if received.Load() == 0 || exported.Load() == 0 {
		t.Fatalf("expected mock collector to receive span, received=%d exported=%d", received.Load(), exported.Load())
	}
	var req otlpTraceExportRequest
	select {
	case req = <-requests:
	default:
		t.Fatal("expected decoded otlp request")
	}
	body := <-bodies
	for _, secret := range []string{
		"TEST_ACCESS_TOKEN_UNIQUE",
		"TEST_REFRESH_TOKEN_UNIQUE",
		"TEST_APP_SECRET_UNIQUE",
		"TEST_COOKIE_UNIQUE",
		"TEST_PHONE_UNIQUE",
		"TEST_EMAIL_UNIQUE",
		"TEST_SIGNED_URL_UNIQUE",
		"TEST_OBJECT_KEY_UNIQUE",
	} {
		if strings.Contains(body, secret) {
			t.Fatalf("sensitive value leaked into otlp request: %s", secret)
		}
	}
	assertStandardOTLPRequest(t, req)
}

func TestHTTPExporterRetriesRetryableStatus(t *testing.T) {
	var attempts atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()
	exp := newHTTPSpanExporter(Config{
		OTLPEndpoint:  srv.URL,
		ExportTimeout: time.Second,
		RetryMax:      1,
	}, nil)
	if err := exp.ExportSpans(context.Background(), testSpanBatch()); err != nil {
		t.Fatal(err)
	}
	if attempts.Load() != 2 {
		t.Fatalf("expected one retry, got %d attempts", attempts.Load())
	}
}

func TestHTTPExporterDoesNotRetryClientStatus(t *testing.T) {
	var attempts atomic.Int64
	var failures atomic.Int64
	provider := &Provider{}
	if provider.ExportAttempted() {
		t.Fatal("new provider must not report an export attempt")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()
	exp := newHTTPSpanExporter(Config{
		OTLPEndpoint:  srv.URL,
		ExportTimeout: time.Second,
		RetryMax:      3,
		OnExportError: func(n int) {
			failures.Add(int64(n))
		},
	}, provider)
	if err := exp.ExportSpans(context.Background(), testSpanBatch()); err == nil {
		t.Fatal("expected exporter error")
	}
	if attempts.Load() != 1 {
		t.Fatalf("expected no retry for 400, got %d attempts", attempts.Load())
	}
	if failures.Load() == 0 {
		t.Fatal("expected export failure callback")
	}
	if !provider.ExportAttempted() || !provider.ExportBlocked() || provider.ExportFailures() != 1 {
		t.Fatalf("expected degraded provider after failed export, attempted=%v blocked=%v failures=%d", provider.ExportAttempted(), provider.ExportBlocked(), provider.ExportFailures())
	}

	recoverySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer recoverySrv.Close()
	exp.endpoint = strings.TrimSuffix(recoverySrv.URL, "/") + "/v1/traces"
	if err := exp.ExportSpans(context.Background(), testSpanBatch()); err != nil {
		t.Fatal(err)
	}
	if !provider.ExportAttempted() || provider.ExportBlocked() {
		t.Fatal("expected a successful export to clear degraded provider state")
	}
	if provider.ExportFailures() != 1 {
		t.Fatalf("successful recovery must preserve cumulative failure count, got %d", provider.ExportFailures())
	}
}

func TestBuildOTLPTraceExportRequestFixtureShape(t *testing.T) {
	req := buildOTLPTraceExportRequest(testSpanBatch())
	raw, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"resourceSpans"`) || !strings.Contains(string(raw), `"scopeSpans"`) {
		t.Fatalf("fixture missing standard otlp fields: %s", raw)
	}
	assertStandardOTLPRequest(t, req)
}

func TestGoldenOTLPFixtureParses(t *testing.T) {
	raw, err := os.ReadFile("testdata/valid_otlp_trace.json")
	if err != nil {
		t.Fatal(err)
	}
	var req otlpTraceExportRequest
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		t.Fatal(err)
	}
	assertStandardOTLPRequest(t, req)
}

func assertStandardOTLPRequest(t *testing.T, req otlpTraceExportRequest) {
	t.Helper()
	if len(req.ResourceSpans) != 1 {
		t.Fatalf("expected one resourceSpans item, got %d", len(req.ResourceSpans))
	}
	rs := req.ResourceSpans[0]
	if !hasAttr(rs.Resource.Attributes, "service.name") {
		t.Fatalf("resource missing service.name: %+v", rs.Resource.Attributes)
	}
	if len(rs.ScopeSpans) == 0 {
		t.Fatal("expected scopeSpans")
	}
	var root, child *otlpSpan
	for i := range rs.ScopeSpans {
		if strings.TrimSpace(rs.ScopeSpans[i].Scope.Name) == "" {
			t.Fatal("scope name is empty")
		}
		for j := range rs.ScopeSpans[i].Spans {
			sp := &rs.ScopeSpans[i].Spans[j]
			if len(sp.TraceID) != 32 {
				t.Fatalf("traceId must be 16 bytes hex, got %q", sp.TraceID)
			}
			if len(sp.SpanID) != 16 {
				t.Fatalf("spanId must be 8 bytes hex, got %q", sp.SpanID)
			}
			if sp.StartTimeUnixNano == "" || sp.EndTimeUnixNano == "" {
				t.Fatalf("span timestamps missing: %+v", sp)
			}
			switch sp.Name {
			case "root-export":
				root = sp
			case "child-export":
				child = sp
			}
		}
	}
	if root == nil || child == nil {
		t.Fatalf("expected root and child spans, root=%v child=%v", root != nil, child != nil)
	}
	if child.ParentSpanID != root.SpanID {
		t.Fatalf("child parent span mismatch: got %s want %s", child.ParentSpanID, root.SpanID)
	}
	if child.Kind != int(trace.SpanKindClient) {
		t.Fatalf("child kind mismatch: %d", child.Kind)
	}
	if child.Status.Code != 2 {
		t.Fatalf("error span status must be OTLP ERROR=2, got %+v", child.Status)
	}
	if !hasAttr(root.Attributes, "app.module") || !hasAttr(root.Attributes, "item.count") || !hasAttr(root.Attributes, "cache.hit") {
		t.Fatalf("typed safe attrs missing: %+v", root.Attributes)
	}
	if hasAttr(root.Attributes, "access_token") || hasAttr(root.Attributes, "signed_url") {
		t.Fatalf("sensitive attrs leaked: %+v", root.Attributes)
	}
}

func hasAttr(attrs []otlpKeyValue, key string) bool {
	for _, attr := range attrs {
		if attr.Key == key {
			return true
		}
	}
	return false
}

func testSpanBatch() []sdktrace.ReadOnlySpan {
	tid, _ := trace.TraceIDFromHex("00112233445566778899aabbccddeeff")
	rootID, _ := trace.SpanIDFromHex("0011223344556677")
	childID, _ := trace.SpanIDFromHex("8899aabbccddeeff")
	now := time.Unix(1700000000, 123)
	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName("trademind-api"),
		semconv.ServiceVersion("test"),
		attribute.String("deployment.environment", "test"),
	)
	stubs := tracetest.SpanStubs{
		{
			Name: "root-export",
			SpanContext: trace.NewSpanContext(trace.SpanContextConfig{
				TraceID: tid,
				SpanID:  rootID,
			}),
			SpanKind:  trace.SpanKindServer,
			StartTime: now,
			EndTime:   now.Add(time.Millisecond),
			Attributes: []attribute.KeyValue{
				attribute.String("app.module", "tracing"),
				attribute.Int64("item.count", 3),
				attribute.Bool("cache.hit", true),
				attribute.String("access_token", "TEST_ACCESS_TOKEN_UNIQUE"),
			},
			Resource:             res,
			InstrumentationScope: instrumentation.Scope{Name: "trademind-test", Version: "v1"},
		},
		{
			Name: "child-export",
			SpanContext: trace.NewSpanContext(trace.SpanContextConfig{
				TraceID: tid,
				SpanID:  childID,
			}),
			Parent:    trace.NewSpanContext(trace.SpanContextConfig{TraceID: tid, SpanID: rootID}),
			SpanKind:  trace.SpanKindClient,
			StartTime: now.Add(time.Millisecond),
			EndTime:   now.Add(2 * time.Millisecond),
			Attributes: []attribute.KeyValue{
				attribute.String("http.method", "GET"),
			},
			Status:               sdktrace.Status{Code: codes.Error, Description: "timeout"},
			Resource:             res,
			InstrumentationScope: instrumentation.Scope{Name: "trademind-test", Version: "v1"},
		},
	}
	return stubs.Snapshots()
}
