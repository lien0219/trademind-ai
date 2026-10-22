package tracing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

// Config holds tracer settings.
type Config struct {
	Enabled       bool
	ServiceName   string
	Version       string
	Environment   string
	SampleRatio   float64
	ExportStdout  bool
	OTLPEndpoint  string
	OTLPProtocol  string
	OTLPHeaders   string
	ExportTimeout time.Duration
	QueueSize     int
	BatchSize     int
	RetryMax      int
	OnExportOK    func(int)
	OnExportError func(int)
	OnQueueDepth  func(int)
}

// Provider wraps OTel tracer provider with safe shutdown.
type Provider struct {
	cfg             Config
	provider        *sdktrace.TracerProvider
	tracer          trace.Tracer
	mu              sync.Mutex
	closed          bool
	exportBlocked   atomic.Bool
	exportAttempted atomic.Bool
	exportFailures  atomic.Int64
}

// Init creates and installs global tracer provider.
func Init(cfg Config) (*Provider, error) {
	p := &Provider{cfg: cfg}
	if !cfg.Enabled {
		otel.SetTracerProvider(trace.NewNoopTracerProvider())
		p.tracer = otel.Tracer(cfg.ServiceName)
		return p, nil
	}
	if cfg.SampleRatio <= 0 {
		cfg.SampleRatio = 0.1
	}
	if cfg.SampleRatio > 1 {
		cfg.SampleRatio = 1
	}
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.Version),
			attribute.String("deployment.environment", cfg.Environment),
		),
	)
	if err != nil {
		return nil, err
	}
	var exporters []sdktrace.SpanExporter
	if cfg.ExportStdout {
		exp, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			return nil, err
		}
		exporters = append(exporters, exp)
	}
	if ep := strings.TrimSpace(cfg.OTLPEndpoint); ep != "" {
		exp := newHTTPSpanExporter(cfg, p)
		exporters = append(exporters, exp)
	}
	var spanExporter sdktrace.SpanExporter
	if len(exporters) == 1 {
		spanExporter = exporters[0]
	} else if len(exporters) > 1 {
		spanExporter = exporters[0]
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SampleRatio))),
	)
	if spanExporter != nil {
		queueSize := boundedQueueSize(cfg.QueueSize)
		batchSize := boundedBatchSize(cfg.BatchSize, queueSize)
		tp = sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(spanExporter,
				sdktrace.WithMaxQueueSize(queueSize),
				sdktrace.WithMaxExportBatchSize(batchSize),
				sdktrace.WithBatchTimeout(2*time.Second),
				sdktrace.WithExportTimeout(exportTimeout(cfg.ExportTimeout)),
			),
			sdktrace.WithResource(res),
			sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SampleRatio))),
		)
	}
	otel.SetTracerProvider(tp)
	p.provider = tp
	p.tracer = tp.Tracer(cfg.ServiceName)
	return p, nil
}

// Tracer returns the application tracer.
func (p *Provider) Tracer() trace.Tracer {
	if p == nil || p.tracer == nil {
		return otel.Tracer("trademind")
	}
	return p.tracer
}

// ExportBlocked reports whether the most recent completed OTLP export failed.
func (p *Provider) ExportBlocked() bool {
	if p == nil {
		return true
	}
	return p.exportBlocked.Load()
}

// ExportAttempted reports whether an OTLP export completed with success or failure.
func (p *Provider) ExportAttempted() bool {
	if p == nil {
		return false
	}
	return p.exportAttempted.Load()
}

// ExportFailures reports exporter failures observed by the lightweight HTTP exporter.
func (p *Provider) ExportFailures() int64 {
	if p == nil {
		return 0
	}
	return p.exportFailures.Load()
}

// Shutdown flushes and shuts down tracer provider.
func (p *Provider) Shutdown(ctx context.Context) error {
	if p == nil || p.provider == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil
	}
	p.closed = true
	return p.provider.Shutdown(ctx)
}

// StartSpan starts a child span with safe attributes only.
func StartSpan(ctx context.Context, tracer trace.Tracer, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	if tracer == nil {
		tracer = otel.Tracer("trademind")
	}
	safe := sanitizeAttrs(attrs)
	return tracer.Start(ctx, name, trace.WithAttributes(safe...))
}

// EndSpan ends span with optional error type.
func EndSpan(span trace.Span, err error, errorType string) {
	if span == nil {
		return
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, safeErrorType(errorType, err))
		if errorType != "" {
			span.SetAttributes(attribute.String("error.type", errorType))
		}
	}
	span.End()
}

func safeErrorType(errorType string, err error) string {
	if strings.TrimSpace(errorType) != "" {
		return errorType
	}
	if err == nil {
		return ""
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "timeout"):
		return "timeout"
	case strings.Contains(msg, "circuit"):
		return "circuit_open"
	default:
		return "error"
	}
}

var forbiddenAttrKeys = []string{
	"authorization", "cookie", "token", "secret", "password", "api_key",
	"prompt", "signed_url", "raw_payload", "phone", "email", "address",
}

func sanitizeAttrs(attrs []attribute.KeyValue) []attribute.KeyValue {
	if len(attrs) == 0 {
		return nil
	}
	out := make([]attribute.KeyValue, 0, len(attrs))
	for _, a := range attrs {
		k := strings.ToLower(string(a.Key))
		blocked := false
		for _, f := range forbiddenAttrKeys {
			if strings.Contains(k, f) {
				blocked = true
				break
			}
		}
		if blocked {
			continue
		}
		out = append(out, a)
	}
	return out
}

// ParseTraceParent parses W3C traceparent header.
func ParseTraceParent(raw string) (trace.TraceID, trace.SpanID, error) {
	raw = strings.TrimSpace(raw)
	parts := strings.Split(raw, "-")
	if len(parts) != 4 || parts[0] != "00" {
		return trace.TraceID{}, trace.SpanID{}, fmt.Errorf("invalid traceparent")
	}
	tid, err := trace.TraceIDFromHex(parts[1])
	if err != nil {
		return trace.TraceID{}, trace.SpanID{}, err
	}
	sid, err := trace.SpanIDFromHex(parts[2])
	if err != nil {
		return trace.TraceID{}, trace.SpanID{}, err
	}
	return tid, sid, nil
}

// FormatTraceParent formats W3C traceparent from span context.
func FormatTraceParent(sc trace.SpanContext) string {
	if !sc.IsValid() {
		return ""
	}
	return fmt.Sprintf("00-%s-%s-01", sc.TraceID().String(), sc.SpanID().String())
}

// LinkParent links consumer span to parent trace context stored as traceparent string.
func LinkParent(ctx context.Context, tracer trace.Tracer, name, traceParent string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	tid, sid, err := ParseTraceParent(traceParent)
	if err != nil {
		return StartSpan(ctx, tracer, name, attrs...)
	}
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: tid,
		SpanID:  sid,
		Remote:  true,
	})
	ctx = trace.ContextWithSpanContext(ctx, sc)
	return StartSpan(ctx, tracer, name, attrs...)
}

type httpSpanExporter struct {
	endpoint string
	headers  http.Header
	client   *http.Client
	cfg      Config
	provider *Provider
}

func newHTTPSpanExporter(cfg Config, provider *Provider) *httpSpanExporter {
	timeout := exportTimeout(cfg.ExportTimeout)
	return &httpSpanExporter{
		endpoint: normalizeEndpoint(cfg.OTLPEndpoint),
		headers:  parseOTLPHeaders(cfg.OTLPHeaders),
		client:   &http.Client{Timeout: timeout},
		cfg:      cfg,
		provider: provider,
	}
}

func (e *httpSpanExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	if e == nil || e.endpoint == "" || len(spans) == 0 {
		return nil
	}
	body, err := json.Marshal(buildOTLPTraceExportRequest(spans))
	if err != nil {
		e.recordFailure(len(spans))
		return err
	}
	var lastErr error
	attempts := retryAttempts(e.cfg.RetryMax)
	for attempt := 0; attempt <= attempts; attempt++ {
		if attempt > 0 {
			if err := sleepBackoff(ctx, attempt); err != nil {
				e.recordFailure(len(spans))
				return err
			}
		}
		err := e.exportOnce(ctx, body, len(spans))
		if err == nil {
			return nil
		}
		lastErr = err
		if !isRetryableExportError(err) {
			break
		}
	}
	e.recordFailure(len(spans))
	return lastErr
}

func (e *httpSpanExporter) exportOnce(ctx context.Context, body []byte, spanCount int) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	for k, vals := range e.headers {
		for _, v := range vals {
			req.Header.Add(k, v)
		}
	}
	resp, err := e.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return exportStatusError(resp.StatusCode)
	}
	if e.cfg.OnExportOK != nil {
		e.cfg.OnExportOK(spanCount)
	}
	if e.cfg.OnQueueDepth != nil {
		e.cfg.OnQueueDepth(0)
	}
	if e.provider != nil {
		e.provider.exportAttempted.Store(true)
		e.provider.exportBlocked.Store(false)
	}
	return nil
}

func (e *httpSpanExporter) Shutdown(ctx context.Context) error {
	_ = ctx
	return nil
}

func (e *httpSpanExporter) recordFailure(n int) {
	if e != nil && e.provider != nil {
		e.provider.exportAttempted.Store(true)
		e.provider.exportBlocked.Store(true)
		e.provider.exportFailures.Add(1)
	}
	if e != nil && e.cfg.OnExportError != nil {
		e.cfg.OnExportError(n)
	}
}

type otlpTraceExportRequest struct {
	ResourceSpans []otlpResourceSpans `json:"resourceSpans"`
}

type otlpResourceSpans struct {
	Resource   otlpResource     `json:"resource"`
	ScopeSpans []otlpScopeSpans `json:"scopeSpans"`
}

type otlpResource struct {
	Attributes []otlpKeyValue `json:"attributes,omitempty"`
}

type otlpScopeSpans struct {
	Scope otlpInstrumentationScope `json:"scope"`
	Spans []otlpSpan               `json:"spans"`
}

type otlpInstrumentationScope struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

type otlpSpan struct {
	TraceID           string         `json:"traceId"`
	SpanID            string         `json:"spanId"`
	ParentSpanID      string         `json:"parentSpanId,omitempty"`
	Name              string         `json:"name"`
	Kind              int            `json:"kind"`
	StartTimeUnixNano string         `json:"startTimeUnixNano"`
	EndTimeUnixNano   string         `json:"endTimeUnixNano"`
	Attributes        []otlpKeyValue `json:"attributes,omitempty"`
	Events            []otlpEvent    `json:"events,omitempty"`
	Status            otlpStatus     `json:"status,omitempty"`
}

type otlpEvent struct {
	TimeUnixNano string         `json:"timeUnixNano"`
	Name         string         `json:"name"`
	Attributes   []otlpKeyValue `json:"attributes,omitempty"`
}

type otlpStatus struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

type otlpKeyValue struct {
	Key   string       `json:"key"`
	Value otlpAnyValue `json:"value"`
}

type otlpAnyValue struct {
	StringValue *string         `json:"stringValue,omitempty"`
	BoolValue   *bool           `json:"boolValue,omitempty"`
	IntValue    *string         `json:"intValue,omitempty"`
	DoubleValue *float64        `json:"doubleValue,omitempty"`
	ArrayValue  *otlpArrayValue `json:"arrayValue,omitempty"`
}

type otlpArrayValue struct {
	Values []otlpAnyValue `json:"values"`
}

func buildOTLPTraceExportRequest(spans []sdktrace.ReadOnlySpan) otlpTraceExportRequest {
	scopeSpans := make(map[string]*otlpScopeSpans)
	resourceAttrs := []attribute.KeyValue{
		semconv.ServiceName("trademind-api"),
	}
	for _, sp := range spans {
		if sp == nil {
			continue
		}
		if res := sp.Resource(); res != nil {
			resourceAttrs = res.Attributes()
		}
		scope := sp.InstrumentationScope()
		scopeName := strings.TrimSpace(scope.Name)
		if scopeName == "" {
			scopeName = "trademind"
		}
		key := scopeName + "\x00" + scope.Version
		bucket := scopeSpans[key]
		if bucket == nil {
			bucket = &otlpScopeSpans{
				Scope: otlpInstrumentationScope{Name: scopeName, Version: scope.Version},
				Spans: make([]otlpSpan, 0, len(spans)),
			}
			scopeSpans[key] = bucket
		}
		bucket.Spans = append(bucket.Spans, spanToOTLP(sp))
	}
	out := otlpResourceSpans{
		Resource:   otlpResource{Attributes: attrsToOTLP(resourceAttrs)},
		ScopeSpans: make([]otlpScopeSpans, 0, len(scopeSpans)),
	}
	for _, ss := range scopeSpans {
		out.ScopeSpans = append(out.ScopeSpans, *ss)
	}
	return otlpTraceExportRequest{ResourceSpans: []otlpResourceSpans{out}}
}

func spanToOTLP(sp sdktrace.ReadOnlySpan) otlpSpan {
	sc := sp.SpanContext()
	status := sp.Status()
	parentSpanID := ""
	if parent := sp.Parent(); parent.IsValid() {
		parentSpanID = parent.SpanID().String()
	}
	return otlpSpan{
		TraceID:           sc.TraceID().String(),
		SpanID:            sc.SpanID().String(),
		ParentSpanID:      parentSpanID,
		Name:              sp.Name(),
		Kind:              int(sp.SpanKind()),
		StartTimeUnixNano: strconv.FormatInt(sp.StartTime().UTC().UnixNano(), 10),
		EndTimeUnixNano:   strconv.FormatInt(sp.EndTime().UTC().UnixNano(), 10),
		Attributes:        attrsToOTLP(sp.Attributes()),
		Events:            eventsToOTLP(sp.Events()),
		Status:            otlpStatus{Code: statusCodeToOTLP(status.Code), Message: safeStatusMessage(status.Description)},
	}
}

func attrsToOTLP(attrs []attribute.KeyValue) []otlpKeyValue {
	safe := sanitizeAttrs(attrs)
	if len(safe) == 0 {
		return nil
	}
	out := make([]otlpKeyValue, 0, len(safe))
	for _, attr := range safe {
		out = append(out, otlpKeyValue{
			Key:   string(attr.Key),
			Value: anyValueToOTLP(attr.Value),
		})
	}
	return out
}

func eventsToOTLP(events []sdktrace.Event) []otlpEvent {
	if len(events) == 0 {
		return nil
	}
	out := make([]otlpEvent, 0, len(events))
	for _, event := range events {
		attrs := attrsToOTLP(event.Attributes)
		if len(attrs) == 0 && shouldDropEventName(event.Name) {
			continue
		}
		out = append(out, otlpEvent{
			TimeUnixNano: strconv.FormatInt(event.Time.UTC().UnixNano(), 10),
			Name:         safeEventName(event.Name),
			Attributes:   attrs,
		})
	}
	return out
}

func anyValueToOTLP(v attribute.Value) otlpAnyValue {
	switch v.Type() {
	case attribute.BOOL:
		b := v.AsBool()
		return otlpAnyValue{BoolValue: &b}
	case attribute.INT64:
		i := strconv.FormatInt(v.AsInt64(), 10)
		return otlpAnyValue{IntValue: &i}
	case attribute.FLOAT64:
		f := v.AsFloat64()
		return otlpAnyValue{DoubleValue: &f}
	case attribute.STRING:
		s := v.AsString()
		return otlpAnyValue{StringValue: &s}
	case attribute.BOOLSLICE:
		vals := make([]otlpAnyValue, 0, len(v.AsBoolSlice()))
		for _, item := range v.AsBoolSlice() {
			b := item
			vals = append(vals, otlpAnyValue{BoolValue: &b})
		}
		return otlpAnyValue{ArrayValue: &otlpArrayValue{Values: vals}}
	case attribute.INT64SLICE:
		vals := make([]otlpAnyValue, 0, len(v.AsInt64Slice()))
		for _, item := range v.AsInt64Slice() {
			i := strconv.FormatInt(item, 10)
			vals = append(vals, otlpAnyValue{IntValue: &i})
		}
		return otlpAnyValue{ArrayValue: &otlpArrayValue{Values: vals}}
	case attribute.FLOAT64SLICE:
		vals := make([]otlpAnyValue, 0, len(v.AsFloat64Slice()))
		for _, item := range v.AsFloat64Slice() {
			f := item
			vals = append(vals, otlpAnyValue{DoubleValue: &f})
		}
		return otlpAnyValue{ArrayValue: &otlpArrayValue{Values: vals}}
	case attribute.STRINGSLICE:
		vals := make([]otlpAnyValue, 0, len(v.AsStringSlice()))
		for _, item := range v.AsStringSlice() {
			s := item
			vals = append(vals, otlpAnyValue{StringValue: &s})
		}
		return otlpAnyValue{ArrayValue: &otlpArrayValue{Values: vals}}
	default:
		s := v.Emit()
		return otlpAnyValue{StringValue: &s}
	}
}

func statusCodeToOTLP(code codes.Code) int {
	switch code {
	case codes.Ok:
		return 1
	case codes.Error:
		return 2
	default:
		return 0
	}
}

func safeStatusMessage(message string) string {
	for _, f := range forbiddenAttrKeys {
		if strings.Contains(strings.ToLower(message), f) {
			return "redacted"
		}
	}
	return message
}

func shouldDropEventName(name string) bool {
	lower := strings.ToLower(name)
	for _, f := range forbiddenAttrKeys {
		if strings.Contains(lower, f) {
			return true
		}
	}
	return false
}

func safeEventName(name string) string {
	if shouldDropEventName(name) {
		return "redacted"
	}
	return name
}

type exportStatusError int

func (e exportStatusError) Error() string {
	return fmt.Sprintf("otlp http exporter status %d", int(e))
}

func isRetryableExportError(err error) bool {
	if err == nil {
		return false
	}
	if status, ok := err.(exportStatusError); ok {
		return status == http.StatusTooManyRequests || int(status) >= 500
	}
	return true
}

func retryAttempts(max int) int {
	if max < 0 {
		return 0
	}
	if max > 5 {
		return 5
	}
	return max
}

func sleepBackoff(ctx context.Context, attempt int) error {
	d := time.Duration(attempt*attempt) * 25 * time.Millisecond
	if d > 250*time.Millisecond {
		d = 250 * time.Millisecond
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func normalizeEndpoint(raw string) string {
	ep := strings.TrimSpace(raw)
	if ep == "" {
		return ""
	}
	if !strings.HasPrefix(ep, "http://") && !strings.HasPrefix(ep, "https://") {
		ep = "http://" + ep
	}
	if strings.HasSuffix(ep, "/") {
		ep = strings.TrimRight(ep, "/")
	}
	if !strings.HasSuffix(ep, "/v1/traces") {
		ep += "/v1/traces"
	}
	return ep
}

func parseOTLPHeaders(raw string) http.Header {
	headers := http.Header{}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		k, v, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		key := http.CanonicalHeaderKey(strings.TrimSpace(k))
		value := strings.TrimSpace(v)
		if key == "" || value == "" || strings.EqualFold(key, "authorization") || strings.EqualFold(key, "cookie") {
			continue
		}
		headers.Add(key, value)
	}
	return headers
}

func exportTimeout(d time.Duration) time.Duration {
	if d <= 0 {
		return 10 * time.Second
	}
	if d > 30*time.Second {
		return 30 * time.Second
	}
	return d
}

func boundedQueueSize(size int) int {
	if size <= 0 {
		return 1024
	}
	if size > 10000 {
		return 10000
	}
	return size
}

func boundedBatchSize(size int, queueSize int) int {
	if size <= 0 {
		size = 128
	}
	if size > queueSize {
		return queueSize
	}
	return size
}
