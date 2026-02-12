package ibp

import (
	"fmt"
	"iter"
	"slices"

	BazelLog "github.com/aspect-build/aspect-gazelle/common/logger"
	"github.com/aspect-build/aspect-gazelle/runner/pkg/socket"
)

type IncrementalClient interface {
	Connect(caps map[WatchCapability]any) error
	Disconnect() error
	AwaitCycle() iter.Seq2[*CycleSourcesMessage, error]
}

type incClient struct {
	socketPath string
	socket     socket.Socket[any, map[string]any]

	// The negotiated protocol version and capabilities
	version ProtocolVersion
	caps    map[WatchCapability]any
}

var _ IncrementalClient = (*incClient)(nil)

func NewClient(host string) IncrementalClient {
	return &incClient{
		socketPath: host,
	}
}
func (c *incClient) Connect(caps map[WatchCapability]any) error {
	if c.socket != nil {
		return fmt.Errorf("client already connected")
	}

	socket, err := socket.ConnectJsonSocket[any, map[string]any](c.socketPath)
	if err != nil {
		return err
	}
	c.socket = socket

	if err := c.negotiate(); err != nil {
		return fmt.Errorf("failed to negotiate protocol version: %w", err)
	}

	if c.version.HasCapMessage() {
		// Send the CAPS message if the server supports it, no matter if/how many caps are requested.
		if err := c.cap(caps); err != nil {
			// Connection setup failed mid-handshake; close/reset so callers can retry.
			_ = c.Disconnect()
			return err
		}
	} else {
		// If CAPS are requested but the server doesn't support capabilities negotiation, return an error.
		if len(caps) > 0 {
			_ = c.Disconnect()
			return fmt.Errorf("server negotiated version %d does not support capabilities negotiation", c.version)
		}
	}

	return nil
}

func (c *incClient) negotiate() error {
	negReq, err := c.socket.Recv()
	if err != nil {
		return err
	}

	if negReq["kind"] != "NEGOTIATE" {
		return fmt.Errorf("Expected NEGOTIATE, got %v", negReq)
	}
	if negReq["versions"] == nil {
		return fmt.Errorf("Received NEGOTIATE without versions: %v", negReq)
	}
	rawVersions, isArray := negReq["versions"].([]any)
	if !isArray {
		return fmt.Errorf("Invalid versions, expected []int, received type: %T", negReq["versions"])
	}

	negotiatedVersion, err := negotiateVersion(rawVersions)
	if err != nil {
		return err
	}

	err = c.socket.Send(negotiateResponseMessage{
		Message: Message{
			Kind: "NEGOTIATE_RESPONSE",
		},
		Version: negotiatedVersion,
	})
	if err != nil {
		return err
	}

	c.version = negotiatedVersion
	return nil
}

func negotiateVersion(acceptedVersions []any) (ProtocolVersion, error) {
	for _, v := range acceptedVersions {
		if slices.Contains(abazelSupportedProtocolVersions, ProtocolVersion(v.(float64))) {
			return ProtocolVersion(v.(float64)), nil
		}
	}
	return -1, fmt.Errorf("unsupported versions %v, expected one of %v", acceptedVersions, abazelSupportedProtocolVersions)
}

func (c *incClient) cap(caps map[WatchCapability]any) error {
	if caps == nil {
		caps = map[WatchCapability]any{}
	}

	err := c.socket.Send(capMessage{
		Message: Message{
			Kind: "CAPS",
		},
		Caps: caps,
	})
	if err != nil {
		return fmt.Errorf("failed to send CAPS request: %w", err)
	}

	r, err := c.socket.Recv()
	if err != nil {
		return fmt.Errorf("failed to receive CAPS response: %w", err)
	}

	if r["kind"] != "CAPS_RESPONSE" {
		return fmt.Errorf("Expected CAPS_RESPONSE, got %v", r)
	}

	c.caps, err = readCapsResponseMap(r["caps"], c.version)
	return err
}

func (c *incClient) Disconnect() error {
	if c.socket == nil {
		return fmt.Errorf("client not connected")
	}

	err := c.socket.Close()
	if err != nil {
		return fmt.Errorf("failed to close socket: %w", err)
	}
	c.socket = nil
	return err
}

func (c *incClient) AwaitCycle() iter.Seq2[*CycleSourcesMessage, error] {
	return func(yield func(*CycleSourcesMessage, error) bool) {
		for {
			msg, err := c.socket.Recv()
			if err != nil {
				fmt.Printf("Error receiving message: %v\n", err)
				yield(nil, err)
				return
			}

			if msg["kind"] == "CYCLE" {
				cycleEvent, cycleErr := convertWireCycle(msg)
				if cycleErr != nil {
					fmt.Printf("Failed read cycle: %v\n", cycleErr)
					continue
				}

				err := c.socket.Send(CycleMessage{
					Message: Message{
						Kind: "CYCLE_STARTED",
					},
					CycleId: cycleEvent.CycleId,
				})
				if err != nil {
					yield(nil, err)
					return
				}

				r := yield(&cycleEvent, nil)

				err = c.socket.Send(CycleMessage{
					Message: Message{
						Kind: "CYCLE_COMPLETED",
					},
					CycleId: cycleEvent.CycleId,
				})
				if err != nil {
					BazelLog.Warnf("Failed to send CYCLE_COMPLETED for cycle_id=%d: %v\n", cycleEvent.CycleId, err)
				}

				if !r {
					return
				}
			} else {
				fmt.Printf("Expected CYCLE, received: %v\n", msg)
				continue
			}
		}
	}
}

func convertWireCycle(msg map[string]any) (CycleSourcesMessage, error) {
	if msg["kind"] != "CYCLE" {
		return CycleSourcesMessage{}, fmt.Errorf("Expected CYCLE, got %v", msg["kind"])
	}

	cycleIdFloat, cycleIdIsFloat := msg["cycle_id"].(float64)
	if !cycleIdIsFloat {
		return CycleSourcesMessage{}, fmt.Errorf("Invalid cycle_id type: %T", msg["cycle_id"])
	}

	cycleId := int(cycleIdFloat)

	sources := make(SourceInfoMap, len(msg["sources"].(map[string]any)))
	for k, v := range msg["sources"].(map[string]any) {
		if v == nil {
			sources[k] = nil
		} else {
			sources[k] = &SourceInfo{
				IsSymlink: readOptionalBool(v.(map[string]any), "is_symlink"),
				IsSource:  readOptionalBool(v.(map[string]any), "is_source"),
			}
		}
	}

	// Scope: default to blank and simply reflect what came over the wire
	// which will depend on the negotiated version and capabilities.
	var scope WatchScope
	if scopeVal, ok := msg["scope"]; ok {
		if scopeStr, ok := scopeVal.(string); ok {
			scope = WatchScope(scopeStr)
		}
	}

	return CycleSourcesMessage{
		CycleMessage: CycleMessage{
			Message: Message{Kind: "CYCLE"},
			CycleId: cycleId,
		},
		Scope:   scope,
		Sources: sources,
	}, nil
}

func readOptionalBool(m map[string]any, key string) *bool {
	if val, ok := m[key]; ok {
		if boolVal, ok := val.(*bool); ok {
			return boolVal
		}
	}
	return nil
}
