package ibp

import (
	"fmt"
	"iter"
	"slices"

	BazelLog "github.com/aspect-build/aspect-gazelle/common/logger"
	"github.com/aspect-build/aspect-gazelle/runner/pkg/socket"
)

type IncrementalClient interface {
	Connect(cap ...WatchCapability) error
	Disconnect() error
	HasCap(cap WatchCapability) bool
	AwaitCycle() iter.Seq2[*CycleSourcesMessage, error]
}

type incClient struct {
	socketPath string
	socket     socket.Socket[interface{}, map[string]interface{}]

	// The negotiated protocol version and capabilities
	version ProtocolVersion
	caps    map[WatchCapability]bool
}

var _ IncrementalClient = (*incClient)(nil)

func NewClient(host string) IncrementalClient {
	return &incClient{
		socketPath: host,
	}
}
func (c *incClient) Connect(cap ...WatchCapability) error {
	if c.socket != nil {
		return fmt.Errorf("client already connected")
	}

	socket, err := socket.ConnectJsonSocket[interface{}, map[string]interface{}](c.socketPath)
	if err != nil {
		return err
	}
	c.socket = socket

	if err := c.negotiate(); err != nil {
		return fmt.Errorf("failed to negotiate protocol version: %w", err)
	}

	if !c.version.HasCapMessage() && len(cap) > 0 {
		return fmt.Errorf("server does not support capabilities negotiation")
	}

	if err := c.cap(cap...); err != nil {
		return fmt.Errorf("failed to setup capabilities: %w", err)
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
	rawVersions, isArray := negReq["versions"].([]interface{})
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
		return fmt.Errorf("failed to negotiate protocol version: %w", err)
	}

	c.version = negotiatedVersion
	return nil
}

func negotiateVersion(acceptedVersions []interface{}) (ProtocolVersion, error) {
	for _, v := range acceptedVersions {
		if slices.Contains(abazelSupportedProtocolVersions, ProtocolVersion(v.(float64))) {
			return ProtocolVersion(v.(float64)), nil
		}
	}
	return -1, fmt.Errorf("unsupported versions %v, expected one of %v", acceptedVersions, abazelSupportedProtocolVersions)
}

func (c *incClient) cap(caps ...WatchCapability) error {
	err := c.socket.Send(capMessage{
		Message: Message{
			Kind: "CAPS",
		},
		Caps: caps,
	})
	if err != nil {
		return fmt.Errorf("failed to negotiate protocol version: %w", err)
	}

	r, err := c.socket.Recv()
	if err != nil {
		return fmt.Errorf("failed to receive CAPS response: %w", err)
	}

	if r["kind"] != "CAPS_RESPONSE" {
		return fmt.Errorf("Expected CAPS_RESPONSE, got %v", r)
	}

	enabledCapsRaw, ok := r["enabled_caps"].([]interface{})
	if !ok {
		return fmt.Errorf("Invalid enabled_caps, expected []string, received type: %T", r["enabled_caps"])
	}

	c.caps = make(map[WatchCapability]bool, len(enabledCapsRaw))
	for _, cap := range enabledCapsRaw {
		c.caps[WatchCapability(cap.(string))] = true
	}

	return nil
}

func (c *incClient) HasCap(cap WatchCapability) bool {
	return c.caps[cap]
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

func convertWireCycle(msg map[string]interface{}) (CycleSourcesMessage, error) {
	if msg["kind"] != "CYCLE" {
		return CycleSourcesMessage{}, fmt.Errorf("Expected CYCLE, got %v", msg["kind"])
	}

	cycleIdFloat, cycleIdIsFloat := msg["cycle_id"].(float64)
	if !cycleIdIsFloat {
		return CycleSourcesMessage{}, fmt.Errorf("Invalid cycle_id type: %T", msg["cycle_id"])
	}

	cycleId := int(cycleIdFloat)

	sources := make(SourceInfoMap, len(msg["sources"].(map[string]interface{})))
	for k, v := range msg["sources"].(map[string]interface{}) {
		if v == nil {
			sources[k] = nil
		} else {
			sources[k] = &SourceInfo{
				IsSymlink: readOptionalBool(v.(map[string]interface{}), "is_symlink"),
				IsSource:  readOptionalBool(v.(map[string]interface{}), "is_source"),
			}
		}
	}

	return CycleSourcesMessage{
		CycleMessage: CycleMessage{
			Message: Message{Kind: "CYCLE"},
			CycleId: cycleId,
		},
		Sources: sources,
	}, nil
}

func readOptionalBool(m map[string]interface{}, key string) *bool {
	if val, ok := m[key]; ok {
		if boolVal, ok := val.(*bool); ok {
			return boolVal
		}
	}
	return nil
}
