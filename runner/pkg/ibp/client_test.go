package ibp

import "testing"

type fakeSocket struct {
	recvQueue []map[string]any
	sent      []any
	closed    bool
}

func (s *fakeSocket) Send(cmd any) error {
	s.sent = append(s.sent, cmd)
	return nil
}

func (s *fakeSocket) Recv() (map[string]any, error) {
	if len(s.recvQueue) == 0 {
		return nil, nil
	}
	n := s.recvQueue[0]
	s.recvQueue = s.recvQueue[1:]
	return n, nil
}

func (s *fakeSocket) Close() error {
	s.closed = true
	return nil
}

func TestCap_SendsCapsAndStoresResponse(t *testing.T) {
	s := &fakeSocket{recvQueue: []map[string]any{{
		"kind": "CAPS_RESPONSE",
		"caps": map[string]any{
			"scope": []any{"sources"},
			"otel":  true,
		},
	}}}

	c := &incClient{socket: s, version: VERSION_1}
	err := c.cap(map[WatchCapability]any{
		WatchCapability_WatchScope: []WatchScope{WatchScope_Sources},
	})
	if err != nil {
		t.Fatalf("cap returned error: %v", err)
	}

	if len(s.sent) != 1 {
		t.Fatalf("expected one sent message, got %d", len(s.sent))
	}
	msg, ok := s.sent[0].(capMessage)
	if !ok {
		t.Fatalf("expected capMessage, got %T", s.sent[0])
	}
	if msg.Kind != "CAPS" {
		t.Fatalf("expected CAPS message kind, got %q", msg.Kind)
	}
	if c.caps[WatchCapability_OTEL] != true {
		t.Fatalf("expected otel cap true, got %#v", c.caps[WatchCapability_OTEL])
	}
}

func TestConvertWireCycle_ParsesScopeAndTraceFields(t *testing.T) {
	msg := map[string]any{
		"kind":     "CYCLE",
		"cycle_id": float64(7),
		"scope":    "sources",
		"trace_id": "trace-123",
		"span_id":  "span-456",
		"sources": map[string]any{
			"a.txt": map[string]any{
				"is_source":  true,
				"is_symlink": false,
			},
			"removed.txt": nil,
		},
	}

	event, err := convertWireCycle(msg)
	if err != nil {
		t.Fatalf("convertWireCycle returned error: %v", err)
	}
	if event.cycleId() != 7 {
		t.Fatalf("expected cycleId() to return 7, got %d", event.cycleId())
	}
	cycle, ok := event.(*CycleSourcesMessage)
	if !ok {
		t.Fatalf("expected *CycleSourcesMessage, got %T", event)
	}

	if cycle.CycleId != 7 {
		t.Fatalf("expected cycle id 7, got %d", cycle.CycleId)
	}
	if cycle.Scope != WatchScope_Sources {
		t.Fatalf("expected scope sources, got %q", cycle.Scope)
	}
	if cycle.TraceId != "trace-123" || cycle.SpanId != "span-456" {
		t.Fatalf("expected trace/span ids to round-trip, got trace=%q span=%q", cycle.TraceId, cycle.SpanId)
	}
	if cycle.Sources["removed.txt"] != nil {
		t.Fatalf("expected removed source to map to nil, got %#v", cycle.Sources["removed.txt"])
	}
	src := cycle.Sources["a.txt"]
	if src == nil {
		t.Fatal("expected non-nil SourceInfo for a.txt")
	}
	if src.IsSource == nil || *src.IsSource != true {
		t.Fatalf("expected IsSource=true, got %v", src.IsSource)
	}
	if src.IsSymlink == nil || *src.IsSymlink != false {
		t.Fatalf("expected IsSymlink=false, got %v", src.IsSymlink)
	}
}

func TestConvertWireCycle_MissingOptionalFieldsDefaultsToEmpty(t *testing.T) {
	msg := map[string]any{
		"kind":     "CYCLE",
		"cycle_id": float64(1),
		"scope":    123,
		"trace_id": 456,
		"span_id":  true,
		"sources":  map[string]any{},
	}

	event, err := convertWireCycle(msg)
	if err != nil {
		t.Fatalf("convertWireCycle returned error: %v", err)
	}
	cycle, ok := event.(*CycleSourcesMessage)
	if !ok {
		t.Fatalf("expected *CycleSourcesMessage, got %T", event)
	}
	if cycle.Scope != "" {
		t.Fatalf("expected empty scope for non-string input, got %q", cycle.Scope)
	}
	if cycle.TraceId != "" || cycle.SpanId != "" {
		t.Fatalf("expected empty trace/span for non-string input, got trace=%q span=%q", cycle.TraceId, cycle.SpanId)
	}
	if cycle.Sources == nil {
		t.Fatalf("expected non-nil Sources for empty-map delta, got nil")
	}
}

func TestConvertWireCycle_CycleResetReturnsCycleResetMessage(t *testing.T) {
	msg := map[string]any{
		"kind":     "CYCLE_RESET",
		"cycle_id": float64(3),
		"trace_id": "trace-9",
	}

	event, err := convertWireCycle(msg)
	if err != nil {
		t.Fatalf("convertWireCycle returned error: %v", err)
	}
	if event.cycleId() != 3 {
		t.Fatalf("expected cycleId() to return 3, got %d", event.cycleId())
	}
	reset, ok := event.(*CycleResetMessage)
	if !ok {
		t.Fatalf("expected *CycleResetMessage for CYCLE_RESET, got %T", event)
	}
	if reset.Kind != "CYCLE_RESET" {
		t.Fatalf("expected kind CYCLE_RESET, got %q", reset.Kind)
	}
	if reset.CycleId != 3 {
		t.Fatalf("expected cycle_id=3, got %d", reset.CycleId)
	}
	if reset.TraceId != "trace-9" {
		t.Fatalf("expected trace_id to round-trip on CYCLE_RESET, got %q", reset.TraceId)
	}
}

func TestConvertWireCycle_RejectsUnknownKind(t *testing.T) {
	msg := map[string]any{
		"kind":     "BOGUS",
		"cycle_id": float64(1),
	}

	if _, err := convertWireCycle(msg); err == nil {
		t.Fatal("expected error for unknown kind, got nil")
	}
}
