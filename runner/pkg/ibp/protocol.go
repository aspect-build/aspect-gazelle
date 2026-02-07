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
	"go.opentelemetry.io/otel/trace"
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

func (v ProtocolVersion) HasScopeCap() bool {
	// Only "version 0" did not have the watch-scope capability within the CAPS message
	return v > 0
}

type WatchCapability string

const (
	WatchCapability_WatchScope WatchCapability = "scope"
	WatchCapability_OTEL       WatchCapability = "otel"
)

type WatchScope string

const (
	WatchScope_Sources  WatchScope = "sources"
	WatchScope_Runfiles WatchScope = "runfiles"
)

const PROTOCOL_SOCKET_ENV = "ABAZEL_WATCH_SOCKET_FILE"

type IncrementalBazel interface {
	// Messaging to the client
	Init(ctx context.Context, scope WatchScope, sources SourceInfoMap) error
	Cycle(ctx context.Context, scope WatchScope, changes SourceInfoMap) error
	Exit(ctx context.Context, err error) error

	// Server + Connection to client
	Serve(ctx context.Context) error
	Close() error
	HasConnection() bool

	WaitForConnection() <-chan ProtocolVersion

	// If this connection is watching the given scope, e.g. sources or runfiles.
	WatchingScope(s WatchScope) bool

	// The path/address a client can connect to.
	Address() string

	// Env variables to provide to clients and potential clients.
	Env() []string
}

type Message struct {
	Kind string `json:"kind"`

	// OTEL trace IDs can be added to any message, omitted if empty.
	TraceId string `json:"trace_id,omitempty"`
	SpanId  string `json:"span_id,omitempty"`
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
	Scope   WatchScope    `json:"scope,omitempty"`
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

func (p *aspectBazelProtocol) WatchingScope(s WatchScope) bool {
	// No caps or no scope cap means default to runfiles only
	if p.caps == nil || p.caps[WatchCapability_WatchScope] == nil {
		return s == WatchScope_Runfiles
	}

	scopes := p.caps[WatchCapability_WatchScope].([]WatchScope)
	return slices.Contains(scopes, s)
}

func (p *aspectBazelProtocol) isOTELCapEnabled() bool {
	if p.caps == nil {
		return false
	}
	return p.caps[WatchCapability_OTEL] == true
}

func (p *aspectBazelProtocol) otelMessage(ctx context.Context, kind string) Message {
	m := Message{Kind: kind}
	if p.isOTELCapEnabled() {
		sc := trace.SpanContextFromContext(ctx)
		if sc.HasTraceID() {
			m.TraceId = sc.TraceID().String()
		}
		if sc.HasSpanID() {
			m.SpanId = sc.SpanID().String()
		}
	}
	return m
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
		if err := p.acceptNegotiation(ctx); err != nil {
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

func (p *aspectBazelProtocol) acceptNegotiation(ctx context.Context) error {
	// Wait for a connection
	if err := p.socket.Accept(); err != nil {
		return err
	}

	// Negotiate the protocol version
	m := negotiateMessage{
		Message:  p.otelMessage(ctx, "NEGOTIATE"),
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
		if err := p.negotiateCapabilities(ctx, version); err != nil {
			return fmt.Errorf("Failed to negotiate capabilities: %v", err)
		}
	}

	p.connectedVersion = version
	p.connectedCh <- version

	return nil
}

func (p *aspectBazelProtocol) negotiateCapabilities(ctx context.Context, version ProtocolVersion) error {
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

	p.caps = caps

	m := capMessage{
		Message: p.otelMessage(ctx, "CAPS_RESPONSE"),
		Caps:    caps,
	}
	if err := p.socket.Send(m); err != nil {
		return fmt.Errorf("Failed to send CAPS_RESPONSE: %v", err)
	}

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

func (p *aspectBazelProtocol) Init(ctx context.Context, scope WatchScope, sources SourceInfoMap) error {
	return p.Cycle(ctx, scope, sources)
}

func (p *aspectBazelProtocol) Cycle(ctx context.Context, scope WatchScope, changes SourceInfoMap) error {
	cycle_id := int(p.cycle_id.Add(1))

	fmt.Printf("%s Sending cycle #%v (%v changes) to %s\n", color.GreenString("INFO:"), cycle_id, len(changes), p.socketPath)

	// Support: Protocol0 did not have the concept of scope so remove it from the message if
	// the connection does not support it to maintain compatibility with older clients.
	if !p.connectedVersion.HasScopeCap() {
		scope = ""
	}

	c := CycleSourcesMessage{
		CycleMessage: CycleMessage{
			Message: p.otelMessage(ctx, "CYCLE"),
			CycleId: cycle_id,
		},
		Scope:   scope,
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

func (p *aspectBazelProtocol) Exit(ctx context.Context, err error) error {
	d := ""
	if err != nil {
		d = err.Error()
	}

	c := exitMessage{
		Message:     p.otelMessage(ctx, "EXIT"),
		Description: d,
	}
	return p.socket.Send(c)
}

func readCapsRequestMap(rawCaps any, version ProtocolVersion) (map[WatchCapability]any, error) {
	caps := map[WatchCapability]any{
		// Defaults based on ProtocolVersion

		// Watch runfiles only by default to align with previous version behaviour
		WatchCapability_WatchScope: []WatchScope{WatchScope_Runfiles},
	}

	if rawCaps == nil {
		return caps, nil
	}

	enabledCapsRaw, ok := rawCaps.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("Invalid caps, expected map[WatchCapability]interface{}, received type: %T", rawCaps)
	}

	for cap, val := range enabledCapsRaw {
		switch WatchCapability(cap) {
		case WatchCapability_WatchScope:
			scopeVal, err := readScopeCapList(val)
			if err != nil {
				return nil, err
			}
			caps[WatchCapability_WatchScope] = scopeVal
		case WatchCapability_OTEL:
			// No additional config for OTEL; it is enabled when this capability is present with a boolean true value.
			caps[WatchCapability_OTEL] = readCapBool(val)
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

func readScopeCapList(val any) ([]WatchScope, error) {
	listVal, isArr := val.([]interface{})
	if !isArr {
		return nil, fmt.Errorf("Invalid value for WatchScope list: %T, expected list", val)
	}

	scopes := make([]WatchScope, 0, len(listVal))
	for _, vAny := range listVal {
		v, isStr := vAny.(string)
		if !isStr {
			return nil, fmt.Errorf("Invalid value for WatchScope capability: %T, expected string", vAny)
		}

		switch WatchScope(v) {
		case WatchScope_Sources, WatchScope_Runfiles:
			scopes = append(scopes, WatchScope(v))
		default:
			return nil, fmt.Errorf("Unknown WatchScope %q, expected %q or %q", v, WatchScope_Sources, WatchScope_Runfiles)
		}
	}

	if len(scopes) == 0 {
		return nil, fmt.Errorf("WatchScope capability must have at least one scope, got empty list")
	}

	return scopes, nil
}

func readCapBool(val any) bool {
	boolVal, isBool := val.(bool)
	if !isBool {
		return false
	}
	return boolVal
}
