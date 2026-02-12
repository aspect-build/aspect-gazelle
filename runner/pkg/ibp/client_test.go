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
	isSource := true
	isSymlink := false
	msg := map[string]any{
		"kind":     "CYCLE",
		"cycle_id": float64(7),
		"scope":    "sources",
		"trace_id": "trace-123",
		"span_id":  "span-456",
		"sources": map[string]any{
			"a.txt": map[string]any{
				"is_source":  &isSource,
				"is_symlink": &isSymlink,
			},
			"removed.txt": nil,
		},
	}

	cycle, err := convertWireCycle(msg)
	if err != nil {
		t.Fatalf("convertWireCycle returned error: %v", err)
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

	cycle, err := convertWireCycle(msg)
	if err != nil {
		t.Fatalf("convertWireCycle returned error: %v", err)
	}
	if cycle.Scope != "" {
		t.Fatalf("expected empty scope for non-string input, got %q", cycle.Scope)
	}
	if cycle.TraceId != "" || cycle.SpanId != "" {
		t.Fatalf("expected empty trace/span for non-string input, got trace=%q span=%q", cycle.TraceId, cycle.SpanId)
	}
}
