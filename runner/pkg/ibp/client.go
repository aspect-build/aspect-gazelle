package ibp

import (
	"fmt"
	"iter"
	"slices"

	BazelLog "github.com/aspect-build/aspect-gazelle/common/logger"
	"github.com/aspect-build/aspect-gazelle/runner/pkg/socket"
)

type WatchClient interface {
	Connect() error
	Disconnect() error
	Subscribe(options WatchOptions) iter.Seq2[*CycleSourcesMessage, error]
}

type incClient struct {
	socketPath string
	socket     socket.Socket[interface{}, map[string]interface{}]

	// The negotiated protocol version
	version ProtocolVersion
}

var _ WatchClient = (*incClient)(nil)

func NewClient(host string) WatchClient {
	return &incClient{
		socketPath: host,
	}
}
func (c *incClient) Connect() error {
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
	for _, v := range slices.Backward(acceptedVersions) {
		if slices.Contains(abazelSupportedProtocolVersions, ProtocolVersion(v.(float64))) {
			return ProtocolVersion(v.(float64)), nil
		}
	}
	return -1, fmt.Errorf("unsupported versions %v, expected one of %v", acceptedVersions, abazelSupportedProtocolVersions)
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

func (c *incClient) Subscribe(options WatchOptions) iter.Seq2[*CycleSourcesMessage, error] {
	return func(yield func(*CycleSourcesMessage, error) bool) {
		// Version 1+ require the initial SUBSCRIBE to start the subscription
		if c.version >= VERSION_1 {
			err := c.socket.Send(SubscribeMessage{
				Message: Message{
					Kind: "SUBSCRIBE",
				},
				WatchType: options.Type,
			})
			if err != nil {
				yield(nil, err)
				return
			}

			msg, err := c.socket.Recv()
			if err != nil {
				yield(nil, err)
				return
			}

			if msg["kind"] != "SUBSCRIBE_RESPONSE" {
				yield(nil, fmt.Errorf("expected SUBSCRIBE_RESPONSE, got %v", msg))
				return
			}
		}

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
