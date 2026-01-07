package ibp

import (
	"context"
	"fmt"
	"os"
	"path"
	"slices"
	"sync/atomic"

	"github.com/aspect-build/aspect-gazelle/runner/pkg/socket"
	"github.com/fatih/color"
)

type ProtocolVersion int

const (
	LEGACY_VERSION_0 ProtocolVersion = 0
	VERSION_1        ProtocolVersion = 1
	LATEST_VERSION   ProtocolVersion = VERSION_1
)

func (v ProtocolVersion) HasCapMessage() bool {
	// Only "version 0" did not have the CAPS message
	return v > 0
}

type WatchCapability string

const PROTOCOL_SOCKET_ENV = "ABAZEL_WATCH_SOCKET_FILE"

type IncrementalBazel interface {
	// Messaging to the client
	Init(sources SourceInfoMap) error
	Cycle(changes SourceInfoMap) error
	Exit(err error) error

	// Server + Connection to client
	Serve(ctx context.Context) error
	Close() error
	HasConnection() bool

	WaitForConnection() <-chan ProtocolVersion

	// The path/address a client can connect to.
	Address() string

	// Env variables to provide to clients and potential clients.
	Env() []string
}

type Message struct {
	Kind string `json:"kind"`
}

type negotiateMessage struct {
	Message
	Versions []ProtocolVersion `json:"versions"`
}
type negotiateResponseMessage struct {
	Message
	Version ProtocolVersion `json:"version"`
}

type capMessage struct {
	Message
	Caps map[WatchCapability]any `json:"caps"`
}

type exitMessage struct {
	Message
	Description string `json:"description"`
}

type SourceInfo struct {
	IsSymlink *bool `json:"is_symlink,omitempty"`
	IsSource  *bool `json:"is_source,omitempty"`

	// TODO: is_directory? mtime? generated?
}
type SourceInfoMap = map[string]*SourceInfo

type CycleMessage struct {
	Message
	CycleId int `json:"cycle_id"`
}

type CycleSourcesMessage struct {
	CycleMessage
	Sources SourceInfoMap `json:"sources"`
}

// The versions supported by this host implementation of the protocol.
// Listed in PRIORITY ORDER, i.e. the first version is the most preferred version to use.
var abazelSupportedProtocolVersions = []ProtocolVersion{
	// Latest+preferred version
	VERSION_1,

	// Fallback for older clients...
	LEGACY_VERSION_0,
}

type aspectBazelSocket = socket.Server[any, map[string]any]

type aspectBazelProtocol struct {
	socket     aspectBazelSocket
	socketPath string

	// connectedCh is used to signal when a connection has been established and the protocol version that was negotiated.
	connectedCh chan ProtocolVersion

	// Available once connectedCh emits a version
	connectedVersion ProtocolVersion
	caps             map[WatchCapability]any

	// cycle_id is used to track the current cycle number.
	cycle_id atomic.Int32
}

var _ IncrementalBazel = (*aspectBazelProtocol)(nil)

func NewServer() IncrementalBazel {
	socketPath := path.Join(os.TempDir(), fmt.Sprintf("aspect-watch-%v-socket", os.Getpid()))
	return &aspectBazelProtocol{
		socketPath: socketPath,
		socket:     socket.NewJsonServer[any, map[string]any](),

		connectedCh:      make(chan ProtocolVersion, 1),
		connectedVersion: -1, // Invalid version to indicate not connected
	}
}

func (p *aspectBazelProtocol) WaitForConnection() <-chan ProtocolVersion {
	return p.connectedCh
}

func (p *aspectBazelProtocol) Env() []string {
	return []string{
		PROTOCOL_SOCKET_ENV + "=" + p.socketPath,
	}
}

func (p *aspectBazelProtocol) Address() string {
	return p.socketPath
}

func (p *aspectBazelProtocol) Serve(ctx context.Context) error {
	if err := p.socket.Serve(p.socketPath); err != nil {
		return err
	}

	go func() {
		// TODO: cancel the if the context is done or if Close() invoked
		if err := p.acceptNegotiation(); err != nil {
			select {
			case <-ctx.Done():
				// Ignore errors if the context is done, as this is likely due to shutdown.
			default:
				// Ignore errors due to no connection being accepted.
				if err == socket.ErrNotAccepted {
					return
				}

				fmt.Printf("%s Failed to connect to aspect bazel protocol at %s: %v\n", color.RedString("ERROR:"), p.socketPath, err)
			}
		}
	}()

	return nil
}

func (p *aspectBazelProtocol) HasConnection() bool {
	return p.socket != nil && p.socket.HasConnection()
}

func (p *aspectBazelProtocol) acceptNegotiation() error {
	// Wait for a connection
	if err := p.socket.Accept(); err != nil {
		return err
	}

	// Negotiate the protocol version
	m := negotiateMessage{
		Message:  Message{Kind: "NEGOTIATE"},
		Versions: abazelSupportedProtocolVersions,
	}
	if err := p.socket.Send(m); err != nil {
		return fmt.Errorf("Failed to send NEGOTIATE: %v", err)
	}

	negResp, err := p.socket.Recv()
	if err != nil {
		return fmt.Errorf("Error receiving NEGOTIATE response: %v", err)
	}

	if negResp["kind"] != "NEGOTIATE_RESPONSE" {
		return fmt.Errorf("Expected NEGOTIATE_RESPONSE, got %v", negResp)
	}
	if negResp["version"] == nil {
		return fmt.Errorf("Received NEGOTIATE_RESPONSE without version: %v", negResp)
	}
	if !slices.Contains(abazelSupportedProtocolVersions, ProtocolVersion(negResp["version"].(float64))) {
		return fmt.Errorf("Received NEGOTIATE_RESPONSE with unsupported version %v, expected one of %v", negResp["version"], abazelSupportedProtocolVersions)
	}

	version := ProtocolVersion(negResp["version"].(float64))

	if version.HasCapMessage() {
		if err := p.negotiateCapabilities(version); err != nil {
			return fmt.Errorf("Failed to negotiate capabilities: %v", err)
		}
	}

	p.connectedVersion = version
	p.connectedCh <- version

	return nil
}

func (p *aspectBazelProtocol) negotiateCapabilities(version ProtocolVersion) error {
	capsResp, err := p.socket.Recv()
	if err != nil {
		return fmt.Errorf("Error receiving CAPS request: %v", err)
	}
	if capsResp["kind"] != "CAPS" {
		return fmt.Errorf("Expected CAPS, got %v", capsResp)
	}

	caps, err := readCapsRequestMap(capsResp["caps"], version)
	if err != nil {
		return fmt.Errorf("Failed to read capabilities: %v", err)
	}

	m := capMessage{
		Message: Message{Kind: "CAPS_RESPONSE"},
		Caps:    caps,
	}
	if err := p.socket.Send(m); err != nil {
		return fmt.Errorf("Failed to send CAPS_RESPONSE: %v", err)
	}

	p.caps = caps

	return nil
}

func (p *aspectBazelProtocol) Close() error {
	if p.socket == nil {
		return nil
	}
	if err := p.socket.Close(); err != nil {
		return err
	}
	p.socket = nil
	return nil
}

func (p *aspectBazelProtocol) Init(sources SourceInfoMap) error {
	return p.Cycle(sources)
}

func (p *aspectBazelProtocol) Cycle(changes SourceInfoMap) error {
	cycle_id := int(p.cycle_id.Add(1))

	fmt.Printf("%s Sending cycle #%v (%v changes) to %s\n", color.GreenString("INFO:"), cycle_id, len(changes), p.socketPath)

	c := CycleSourcesMessage{
		CycleMessage: CycleMessage{
			Message: Message{Kind: "CYCLE"},
			CycleId: cycle_id,
		},
		Sources: changes,
	}
	if err := p.socket.Send(c); err != nil {
		return err
	}

	for {
		resp, err := p.socket.Recv()
		if err != nil {
			return err
		}

		receivedCycleId, ok := resp["cycle_id"].(float64)
		if !ok {
			return fmt.Errorf("Received response with invalid cycle_id type: %v", resp)
		}

		if int(receivedCycleId) != cycle_id {
			return fmt.Errorf("Received unexpected cycle response to cycle_id=%d: %v", cycle_id, resp)
		}

		switch resp["kind"] {
		// Still pending events
		case "CYCLE_STARTED":
			continue

		// End events
		case "CYCLE_ABORTED":
			fallthrough
		case "CYCLE_FAILED":
			fmt.Printf("%s received %v event: %v\n", color.RedString("ERROR:"), resp["kind"], resp)
			return nil

		case "CYCLE_COMPLETED":
			return nil

		default:
			return fmt.Errorf("Received unexpected response kind %v for cycle_id=%d: %v", resp["kind"], cycle_id, resp)
		}
	}
}

func (p *aspectBazelProtocol) Exit(err error) error {
	d := ""
	if err != nil {
		d = err.Error()
	}

	c := exitMessage{
		Message:     Message{Kind: "EXIT"},
		Description: d,
	}
	return p.socket.Send(c)
}

func readCapsRequestMap(rawCaps any, version ProtocolVersion) (map[WatchCapability]any, error) {
	caps := map[WatchCapability]any{
		// Defaults based on ProtocolVersion
	}

	if rawCaps == nil {
		return caps, nil
	}

	enabledCapsRaw, ok := rawCaps.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("Invalid caps, expected map[WatchCapability]interface{}, received type: %T", rawCaps)
	}

	for cap, val := range enabledCapsRaw {
		switch cap {
		default:
			// Unknown capabilities are not processed, may still be accessed by the sdk user.
			caps[WatchCapability(cap)] = val
		}
	}

	return caps, nil
}

func readCapsResponseMap(rawCaps any, version ProtocolVersion) (map[WatchCapability]any, error) {
	// Today the caps response map is the same as the request, simple deserialization+validation
	return readCapsRequestMap(rawCaps, version)
}
