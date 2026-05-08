package ibp

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

type fakeServerSocket struct {
	recvQueue   []map[string]any
	sent        []any
	acceptCalls int
	recvCalls   int
}

func (s *fakeServerSocket) Send(cmd any) error {
	s.sent = append(s.sent, cmd)
	return nil
}

func (s *fakeServerSocket) Recv() (map[string]any, error) {
	s.recvCalls++
	if len(s.recvQueue) == 0 {
		return nil, nil
	}
	r := s.recvQueue[0]
	s.recvQueue = s.recvQueue[1:]
	return r, nil
}

func (s *fakeServerSocket) Close() error {
	return nil
}

func (s *fakeServerSocket) Serve(path string) error {
	return nil
}

func (s *fakeServerSocket) Accept() error {
	s.acceptCalls++
	return nil
}

func (s *fakeServerSocket) HasConnection() bool {
	return true
}

func TestReadCapsRequestMap_DefaultsToRunfilesWhenNil(t *testing.T) {
	caps, err := readCapsRequestMap(nil, VERSION_1)
	if err != nil {
		t.Fatalf("readCapsRequestMap(nil) returned error: %v", err)
	}

	scopes, ok := caps[WatchCapability_WatchScope].([]WatchScope)
	if !ok {
		t.Fatalf("scope cap missing or wrong type: %#v", caps[WatchCapability_WatchScope])
	}
	if len(scopes) != 1 || scopes[0] != WatchScope_Runfiles {
		t.Fatalf("expected default [runfiles], got %#v", scopes)
	}
}

func TestReadCapsRequestMap_ParsesKnownAndUnknownCaps(t *testing.T) {
	raw := map[string]any{
		"scope": []any{"sources", "runfiles"},
		"otel":  true,
		"x":     "value",
	}

	caps, err := readCapsRequestMap(raw, VERSION_1)
	if err != nil {
		t.Fatalf("readCapsRequestMap returned error: %v", err)
	}

	scopes := caps[WatchCapability_WatchScope].([]WatchScope)
	if len(scopes) != 2 || scopes[0] != WatchScope_Sources || scopes[1] != WatchScope_Runfiles {
		t.Fatalf("unexpected scopes: %#v", scopes)
	}
	if caps[WatchCapability_OTEL] != true {
		t.Fatalf("expected otel=true, got %#v", caps[WatchCapability_OTEL])
	}
	if caps[WatchCapability("x")] != "value" {
		t.Fatalf("expected unknown cap passthrough, got %#v", caps[WatchCapability("x")])
	}
}

func TestReadCapsRequestMap_RejectsInvalidScope(t *testing.T) {
	_, err := readCapsRequestMap(map[string]any{"scope": []any{}}, VERSION_1)
	if err == nil {
		t.Fatal("expected error for empty scope list")
	}
}

func TestWatchingScope_DefaultAndConfigured(t *testing.T) {
	p := &aspectBazelProtocol{}
	if !p.WatchingScope(WatchScope_Runfiles) {
		t.Fatal("expected default to watch runfiles")
	}
	if p.WatchingScope(WatchScope_Sources) {
		t.Fatal("did not expect default to watch sources")
	}

	p.caps = map[WatchCapability]any{
		WatchCapability_WatchScope: []WatchScope{WatchScope_Sources},
	}
	if !p.WatchingScope(WatchScope_Sources) {
		t.Fatal("expected sources scope to be watched")
	}
	if p.WatchingScope(WatchScope_Runfiles) {
		t.Fatal("did not expect runfiles scope to be watched")
	}
}

func TestOtelMessage_IncludesIDsOnlyWhenCapabilityEnabled(t *testing.T) {
	traceID, err := trace.TraceIDFromHex("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("trace id parse failed: %v", err)
	}
	spanID, err := trace.SpanIDFromHex("0123456789abcdef")
	if err != nil {
		t.Fatalf("span id parse failed: %v", err)
	}
	ctx := trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID,
		SpanID:  spanID,
	}))

	p := &aspectBazelProtocol{}
	m := p.otelMessage(ctx, "CYCLE")
	if m.TraceId != "" || m.SpanId != "" {
		t.Fatalf("expected no otel ids without cap, got trace=%q span=%q", m.TraceId, m.SpanId)
	}

	p.caps = map[WatchCapability]any{WatchCapability_OTEL: true}
	m = p.otelMessage(ctx, "CYCLE")
	if m.TraceId != traceID.String() || m.SpanId != spanID.String() {
		t.Fatalf("expected otel ids, got trace=%q span=%q", m.TraceId, m.SpanId)
	}
}

func TestAcceptNegotiation_LegacyVersionSkipsCapsHandshake(t *testing.T) {
	socket := &fakeServerSocket{
		recvQueue: []map[string]any{
			{
				"kind":    "NEGOTIATE_RESPONSE",
				"version": float64(LEGACY_VERSION_0),
			},
		},
	}
	p := &aspectBazelProtocol{
		socket:           socket,
		connectedCh:      make(chan ProtocolVersion, 1),
		connectedVersion: -1,
	}

	if err := p.acceptNegotiation(context.Background()); err != nil {
		t.Fatalf("acceptNegotiation returned error: %v", err)
	}
	if socket.acceptCalls != 1 {
		t.Fatalf("expected one accept call, got %d", socket.acceptCalls)
	}
	if socket.recvCalls != 1 {
		t.Fatalf("expected one recv call for legacy negotiation, got %d", socket.recvCalls)
	}
	if len(socket.sent) != 1 {
		t.Fatalf("expected one sent message, got %d", len(socket.sent))
	}
	msg, ok := socket.sent[0].(negotiateMessage)
	if !ok {
		t.Fatalf("expected negotiateMessage, got %T", socket.sent[0])
	}
	if msg.Kind != "NEGOTIATE" {
		t.Fatalf("expected NEGOTIATE kind, got %q", msg.Kind)
	}
	if p.connectedVersion != LEGACY_VERSION_0 {
		t.Fatalf("expected connected version 0, got %d", p.connectedVersion)
	}
	if p.caps != nil {
		t.Fatalf("expected no caps for legacy negotiation, got %#v", p.caps)
	}

	select {
	case version := <-p.connectedCh:
		if version != LEGACY_VERSION_0 {
			t.Fatalf("expected legacy version on connectedCh, got %d", version)
		}
	default:
		t.Fatal("expected connectedCh to receive negotiated legacy version")
	}
}

func TestCycle_LegacyVersionClearsScopeForCompatibility(t *testing.T) {
	socket := &fakeServerSocket{
		recvQueue: []map[string]any{
			{
				"kind":     "CYCLE_COMPLETED",
				"cycle_id": float64(1),
			},
		},
	}
	p := &aspectBazelProtocol{
		socket:           socket,
		socketPath:       "test.sock",
		connectedVersion: LEGACY_VERSION_0,
	}

	if err := p.Cycle(context.Background(), WatchScope_Sources, SourceInfoMap{}); err != nil {
		t.Fatalf("Cycle returned error: %v", err)
	}
	if len(socket.sent) != 1 {
		t.Fatalf("expected one CYCLE send, got %d", len(socket.sent))
	}

	msg, ok := socket.sent[0].(CycleSourcesMessage)
	if !ok {
		t.Fatalf("expected CycleSourcesMessage, got %T", socket.sent[0])
	}
	if msg.Scope != "" {
		t.Fatalf("expected empty scope for legacy protocol, got %q", msg.Scope)
	}
}

func TestCycle_V1KeepsScope(t *testing.T) {
	socket := &fakeServerSocket{
		recvQueue: []map[string]any{
			{
				"kind":     "CYCLE_COMPLETED",
				"cycle_id": float64(1),
			},
		},
	}
	p := &aspectBazelProtocol{
		socket:           socket,
		socketPath:       "test.sock",
		connectedVersion: VERSION_1,
	}

	if err := p.Cycle(context.Background(), WatchScope_Sources, SourceInfoMap{}); err != nil {
		t.Fatalf("Cycle returned error: %v", err)
	}

	msg, ok := socket.sent[0].(CycleSourcesMessage)
	if !ok {
		t.Fatalf("expected CycleSourcesMessage, got %T", socket.sent[0])
	}
	if msg.Scope != WatchScope_Sources {
		t.Fatalf("expected sources scope for v1 protocol, got %q", msg.Scope)
	}
	if msg.Kind != "CYCLE" {
		t.Fatalf("expected CYCLE kind, got %q", msg.Kind)
	}
}

func TestCycleReset_V2SendsCycleResetMessage(t *testing.T) {
	socket := &fakeServerSocket{
		recvQueue: []map[string]any{
			{
				"kind":     "CYCLE_COMPLETED",
				"cycle_id": float64(1),
			},
		},
	}
	p := &aspectBazelProtocol{
		socket:           socket,
		socketPath:       "test.sock",
		connectedVersion: VERSION_2,
	}

	if err := p.CycleReset(context.Background()); err != nil {
		t.Fatalf("CycleReset returned error: %v", err)
	}

	if len(socket.sent) != 1 {
		t.Fatalf("expected one send, got %d", len(socket.sent))
	}
	msg, ok := socket.sent[0].(CycleResetMessage)
	if !ok {
		t.Fatalf("expected CycleResetMessage, got %T", socket.sent[0])
	}
	if msg.Kind != "CYCLE_RESET" {
		t.Fatalf("expected CYCLE_RESET kind, got %q", msg.Kind)
	}
	if msg.CycleId != 1 {
		t.Fatalf("expected cycle_id=1, got %d", msg.CycleId)
	}
}

func TestCycleReset_RejectsOnPreV2Connection(t *testing.T) {
	for _, version := range []ProtocolVersion{LEGACY_VERSION_0, VERSION_1} {
		p := &aspectBazelProtocol{
			socket:           &fakeServerSocket{},
			socketPath:       "test.sock",
			connectedVersion: version,
		}
		if err := p.CycleReset(context.Background()); err == nil {
			t.Fatalf("expected CycleReset to fail on v%d, got nil error", version)
		}
	}
}

func TestCycleResetMessage_SerializesWithoutSourcesOrScope(t *testing.T) {
	msg := CycleResetMessage{
		CycleMessage: CycleMessage{
			Message: Message{Kind: "CYCLE_RESET"},
			CycleId: 7,
		},
	}
	b, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal returned error: %v", err)
	}
	if !bytes.Contains(b, []byte(`"kind":"CYCLE_RESET"`)) {
		t.Fatalf("expected kind=CYCLE_RESET, got %s", b)
	}
	if bytes.Contains(b, []byte(`"sources"`)) {
		t.Fatalf("CYCLE_RESET must not carry sources, got %s", b)
	}
	if bytes.Contains(b, []byte(`"scope"`)) {
		t.Fatalf("CYCLE_RESET must not carry scope, got %s", b)
	}
}

func TestInit_SendsBaselineCycle(t *testing.T) {
	socket := &fakeServerSocket{
		recvQueue: []map[string]any{
			{
				"kind":     "CYCLE_COMPLETED",
				"cycle_id": float64(1),
			},
		},
	}
	p := &aspectBazelProtocol{
		socket:           socket,
		socketPath:       "test.sock",
		connectedVersion: VERSION_1,
	}

	if err := p.Init(context.Background(), WatchScope_Sources, SourceInfoMap{"a.txt": nil}); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}

	msg, ok := socket.sent[0].(CycleSourcesMessage)
	if !ok {
		t.Fatalf("expected CycleSourcesMessage, got %T", socket.sent[0])
	}
	if msg.Kind != "CYCLE" {
		t.Fatalf("expected Init to emit a CYCLE (not CYCLE_RESET), got %q", msg.Kind)
	}
	if _, ok := msg.Sources["a.txt"]; !ok {
		t.Fatalf("expected Init to forward baseline sources, got %#v", msg.Sources)
	}
}
