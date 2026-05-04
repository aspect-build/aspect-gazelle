package watchman

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"maps"
	"os"
	"os/exec"
	"path"
	"sync/atomic"

	"github.com/aspect-build/aspect-gazelle/common/bazel"
	"github.com/aspect-build/aspect-gazelle/runner/pkg/socket"
)

type ChangeSet struct {
	// Workspace Relative paths of changed files. eg: ["src/urchin/urchin.go", "src/urchin/urchin_test.go"]
	Paths []string
	// Root of the workspace. eg: /Users/thesayyn/Documents/urchin
	Root string
	// ClockSpec is a point in time that represents the state of the filesystem
	// you could use this to query changes since then but DO NOT rely on the specifics
	// of this string, treat it as an opaque token
	ClockSpec string

	// Fresh instance means that the watchman daemon has no prior knowledge of the state of the filesystem
	// and is starting from scratch.
	//
	// Normally watchman would report all the files in the workspace as changed in this case but we set the
	// `empty_on_fresh_instance` parameter to true in the query to avoid this because we want to traverse the
	// filesystem ourselves. In the future we mgiht to rely on watchman to get initial state of the filesystem
	// instead of traversing it ourselves.
	//
	// Here are some cases where `IsFreshInstance` might be true:
	//
	// 1. The watchman daemon was restarted since your last query
	// 2. The workspace watch was cancelled and restarted.
	// 3. Watchman is not able to track the changes
	// 4. The system was unable to keep up with the rate of change of watched files and the kernel flushed the queue to keep up.
	//    A recrawl of the filesystem by watchman to re-examine the watched tree to determine the current state.
	// 5. You're using timestamps rather than clocks and the timestamp is out of range of known events.
	// 6. You're using a `named cursor`` and that name has not been used before.
	// 7. You're using a blank clock string for the since generator in a query (this is not the same thing as a since term in a query expression!)
	//
	// IMPORTANT: IsFreshInstance ought to indicate a cache discard and a full traversal of the filesystem.
	IsFreshInstance bool
}

type SubscribeOptions interface {
	apply(o map[string]any)
}

type DropState struct {
	// See: https://facebook.github.io/watchman/docs/cmd/subscribe#drop
	DropWithinState string
}

var _ SubscribeOptions = (*DropState)(nil)

func (d DropState) apply(o map[string]any) {
	o["drop"] = []string{d.DropWithinState}
}

type DeferState struct {
	// See: https://facebook.github.io/watchman/docs/cmd/subscribe#defer
	DeferWithinState string
}

var _ SubscribeOptions = (*DeferState)(nil)

func (d DeferState) apply(o map[string]any) {
	o["defer"] = []string{d.DeferWithinState}
}

// Watcher is an interface that abstracts the underlying filesystem watching mechanism
type Watcher interface {
	Start() error
	Stop() error
	GetDiff(clockspec string) (*ChangeSet, error)
	Subscribe(ctx context.Context, options ...SubscribeOptions) iter.Seq2[*ChangeSet, error]
	Close() error
}

// controlResponse unions the PDU shapes received on the control socket
// (watch-project, clock, query, watch-del, state-enter, state-leave).
type controlResponse struct {
	Error string `json:"error"`

	// watch-project
	Watch        string `json:"watch"`
	RelativePath string `json:"relative_path"`

	// clock + query
	Clock           string   `json:"clock"`
	Files           []string `json:"files"`
	IsFreshInstance bool     `json:"is_fresh_instance"`

	// watch-del. *bool to distinguish absent (nil) from explicit false.
	WatchDel *bool `json:"watch-del"`

	// state-enter / state-leave (value is the state name)
	StateEnter *string `json:"state-enter"`
	StateLeave *string `json:"state-leave"`
}

// subscribeResponse unions the PDU shapes received on a subscribe socket
// (initial response, change notifications, state PDUs, stream-end PDUs).
type subscribeResponse struct {
	Error string `json:"error"`

	// Initial subscribe response (carries the subscription name back).
	Subscribe string `json:"subscribe"`

	// Change notification
	Clock           string               `json:"clock"`
	Root            string               `json:"root"`
	Files           []subscribeFileEntry `json:"files"`
	IsFreshInstance bool                 `json:"is_fresh_instance"`

	// Stream-end PDUs. *bool = presence test.
	Unsubscribe *bool `json:"unsubscribe"`
	Canceled    *bool `json:"canceled"`

	// Unilateral state PDUs (value is the state name).
	StateEnter *string `json:"state-enter"`
	StateLeave *string `json:"state-leave"`
}

// subscribeFileEntry is one entry in a Subscribe `files` array (object form,
// because Subscribe requests `fields: ["name", "content.sha1hex"]`).
//
// Sha1Hex is RawMessage because watchman emits either a hex string or an
// error object like `{"error": "..."}` when hashing failed/was unavailable.
type subscribeFileEntry struct {
	Name    string          `json:"name"`
	Sha1Hex json.RawMessage `json:"content.sha1hex"`
}

type watchmanSocket = socket.Socket[[]any, controlResponse]
type subscribeSocketType = socket.Socket[[]any, subscribeResponse]

type WatchmanWatcher struct {
	// Root of the bazel workspace being watched
	workspaceDir string

	// Watchman socket
	socket watchmanSocket
	// Path to watchman executable
	watchmanPath string
	// Last clockspec used in the query
	lastClockSpec string
	// Atomic counter for incrementing subscriber IDs for every call to Subscribe
	subscriberId atomic.Uint64

	// The root of the watchman project
	watchedRoot string

	// The relative path from the watchman project to the worksapce root
	watchedRelPath string
}

func (w *WatchmanWatcher) makeQueryParams(clockspec string, includeHash bool) map[string]any {
	bazelignoreDirs, err := bazel.LoadBazelIgnore(w.workspaceDir)
	if err != nil {
		fmt.Printf("failed to load bazelignore: %v", err)
	}

	bazelignoreDirnameExpressions := make([]any, 0, len(bazelignoreDirs))
	for _, ignoredDir := range bazelignoreDirs {
		bazelignoreDirnameExpressions = append(bazelignoreDirnameExpressions, []any{
			"dirname", ignoredDir,
		})
	}

	var fields []string
	if includeHash {
		fields = []string{"name", "content.sha1hex"}
	} else {
		fields = []string{"name"}
	}

	var queryParams = map[string]any{
		"fields": fields,
		// Avoid an unnecessarily long response on the first query by omitting the list of potentially
		// changed (thus at that point, all) files.
		// See ChangeSet.IsFreshInstance for more information.
		// FR: maybe stop gazelle from traversing the filesystem on the first query and use this instead.
		"empty_on_fresh_instance": true,

		"relative_root": w.watchedRelPath,
		"expression": []any{
			"not",
			append(
				[]any{
					"anyof",
					// maybe not exclude directories? or just report directories to determine what directories have changed?
					[]any{
						"type", "d",
					},
				},
				bazelignoreDirnameExpressions...,
			),
		},
		"ignore_dirs": bazelignoreDirs,
	}

	if clockspec != "" {
		queryParams["since"] = clockspec
	}

	return queryParams
}

func (w *WatchmanWatcher) findWatchman() error {
	if w.watchmanPath != "" {
		return nil
	}
	p, err := exec.LookPath("watchman")
	if err != nil {
		// FR: automatically install watchman if not found
		return fmt.Errorf("watchman not found in PATH: %w, did you install it?", err)
	}
	w.watchmanPath = p
	return nil
}

func (w *WatchmanWatcher) getWatchmanSocket() (string, error) {
	if err := w.findWatchman(); err != nil {
		return "", err
	}
	cmd := exec.Command(w.watchmanPath, "get-sockname")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("watchman get-sockname failed: %w", err)
	}

	var sockname map[string]string
	if err := json.Unmarshal(out, &sockname); err != nil {
		return "", fmt.Errorf("failed to parse get-socketname output: %w", err)
	}

	socketPath := sockname["sockname"]
	if socketPath == "" {
		return "", fmt.Errorf("watchman socket not found")
	}
	return socketPath, nil
}

func (w *WatchmanWatcher) connect() (watchmanSocket, error) {
	return connectTyped[controlResponse](w)
}

func (w *WatchmanWatcher) connectSubscribe() (subscribeSocketType, error) {
	return connectTyped[subscribeResponse](w)
}

// connectTyped is a free function because Go methods can't have type parameters.
func connectTyped[R any](w *WatchmanWatcher) (socket.Socket[[]any, R], error) {
	sockname, err := w.getWatchmanSocket()
	if err != nil {
		return nil, fmt.Errorf("failed to get watchman socket: %w", err)
	}
	sock, err := socket.ConnectJsonSocket[[]any, R](sockname)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to watchman socket: %w", err)
	}
	return sock, nil
}

func (w *WatchmanWatcher) recv() (controlResponse, error) {
	if w.socket == nil {
		return controlResponse{}, fmt.Errorf("watchman socket closed")
	}
	return w.socket.Recv()
}

func (w *WatchmanWatcher) send(args ...any) error {
	if w.socket == nil {
		return fmt.Errorf("watchman socket closed")
	}
	return w.socket.Send(args)
}

// If clockspec is nil, it will return the changes since the last call to GetDiff
// If clockspec is not nil, it will return the changes since the provided clockspec
func (w *WatchmanWatcher) GetDiff(clockspec string) (*ChangeSet, error) {
	if w.socket == nil {
		return nil, fmt.Errorf("watchman socket closed")
	}
	if clockspec == "" {
		clockspec = w.lastClockSpec
	}
	if err := w.send("query", w.watchedRoot, w.makeQueryParams(clockspec, false)); err != nil {
		return nil, fmt.Errorf("failed to send query command: %w", err)
	}

	resp, err := w.recv()
	if err != nil {
		return nil, fmt.Errorf("failed to receive query response: %w", err)
	}

	// Check for error in query response
	// https://facebook.github.io/watchman/docs/cmd/query#response
	if resp.Error != "" {
		return nil, fmt.Errorf("query error response: %s", resp.Error)
	}

	files := resp.Files
	if files == nil {
		files = []string{}
	}
	w.lastClockSpec = resp.Clock

	return &ChangeSet{
		Paths:           files,
		Root:            w.workspaceDir,
		ClockSpec:       w.lastClockSpec,
		IsFreshInstance: resp.IsFreshInstance,
	}, nil
}

// Connects to the watchman socket and starts watching the workspace
//
// Calling start multiple times will not start multiple watches
func (w *WatchmanWatcher) Start() error {
	if w.socket != nil {
		return nil
	}

	socket, err := w.connect()
	if err != nil {
		return err
	}
	w.socket = socket

	if err := w.send("watch-project", w.workspaceDir); err != nil {
		return fmt.Errorf("failed to send watch-project command: %w", err)
	}

	resp, err := w.recv()
	if err != nil {
		return fmt.Errorf("failed to receive watch-project response: %w", err)
	}

	if resp.Error != "" {
		return fmt.Errorf("watch-project error response: %s", resp.Error)
	}

	w.watchedRoot = resp.Watch
	w.watchedRelPath = resp.RelativePath

	if err := w.send("clock", w.watchedRoot); err != nil {
		return fmt.Errorf("failed to send clock command: %w", err)
	}

	resp, err = w.recv()
	if err != nil {
		return fmt.Errorf("failed to receive clock response: %w", err)
	}
	if resp.Clock == "" {
		return fmt.Errorf("failed to get clock: %+v", resp)
	}

	w.lastClockSpec = resp.Clock

	return nil
}

// Stop watching the workspace if it was previously started, NO-OP if it was not started
//
// Do not call this function if you wish to resume watching the workspace at a later time.
//
// NOTE: This will not close any activate subscriptions, refer to the Subscribe function for that.
func (w *WatchmanWatcher) Stop() error {
	w.lastClockSpec = ""
	if err := w.send("watch-del", w.watchedRoot); err != nil {
		return fmt.Errorf("failed to send watch-del command: %w", err)
	}
	// Receive and validate watch-del response
	// https://facebook.github.io/watchman/docs/cmd/watch-del#response
	rsp, err := w.recv()
	if err != nil {
		return fmt.Errorf("failed to receive watch-del response: %w", err)
	}
	if rsp.WatchDel == nil || !*rsp.WatchDel {
		return fmt.Errorf("unknown watch-del response: %+v", rsp)
	}
	return nil
}

// This does not stop watching the workspace so next time it will resume where it left off.
//
// NOTE: This will not close any activate subscriptions, refer to the Subscribe function for that.
func (w *WatchmanWatcher) Close() error {
	if w.socket == nil {
		return nil
	}

	err := w.socket.Close()
	w.socket = nil
	return err
}

// This starts a new socket connection and starts watching the workspace
// Its important to note that ChangeSets received here will not move the
// lastClockSpec forward for the GetDiff function as this is a separate
// mechanism for receiving changes.
//
// Always the first ChangeSet will be a changeset with no changes to indicate
// the initial state of the workspace. In the future we might report current
// state of the filesystem instead of an empty changeset.
//
// See DropState, DeferState for options you can pass to modify subscription behavior.
func (w *WatchmanWatcher) Subscribe(ctx context.Context, options ...SubscribeOptions) iter.Seq2[*ChangeSet, error] {
	return func(yield func(*ChangeSet, error) bool) {
		if w.socket == nil {
			yield(nil, fmt.Errorf("watcher not started, call Start() first"))
			return
		}

		// Create a child context that we can cancel to ensure goroutine cleanup
		ctxCancel, cancel := context.WithCancel(ctx)
		defer cancel() // Ensure the goroutine exits when the iterator completes

		sock, err := w.connectSubscribe()
		if err != nil {
			yield(nil, err)
			return
		}

		// Close the socket when the iterator is complete (immediate cleanup)
		defer sock.Close()

		// Close the socket on context cancellation to interrupt blocking I/O operations
		go func() {
			<-ctxCancel.Done()
			sock.Close() // Unblock any pending Recv/Send calls
		}()

		subscriptionName := fmt.Sprintf("aspect-cli-%d.%d", os.Getpid(), w.subscriberId.Add(1))
		queryParams := w.makeQueryParams(w.lastClockSpec, true)
		for _, option := range options {
			option.apply(queryParams)
		}

		err = sock.Send([]any{"subscribe", w.watchedRoot, subscriptionName, queryParams})
		if err != nil {
			yield(nil, fmt.Errorf("failed to send subscribe command: %w", err))
			return
		}

		resp, err := sock.Recv()
		if err != nil {
			yield(nil, fmt.Errorf("failed to receive subscribe response: %w", err))
			return
		}

		if resp.Error != "" {
			yield(nil, fmt.Errorf("failed to subscribe to project: %s", resp.Error))
			return
		}

		if resp.Subscribe != subscriptionName {
			yield(nil, fmt.Errorf("wrong subscription name: %q != %q", resp.Subscribe, subscriptionName))
			return
		}

		// NOTE: this initial subscribe response will NOT contain the initial
		// "files" because of the "empty_on_fresh_instance" parameter.
		// This also means "is_fresh_instance" is irrelevant and essentially always true.

		// BEST EFFORT: if the subscriber panics, try to unsubscribe from watchman
		defer sock.Send([]any{"unsubscribe", w.watchedRoot, subscriptionName})

		var prevDiffHashes map[string]string
		for {
			resp, err := sock.Recv()
			if err != nil {
				yield(nil, fmt.Errorf("failed to receive watchman response: %w", err))
				return
			}

			// There was an error. Yield the error and stop.
			if resp.Error != "" {
				yield(nil, fmt.Errorf("watchman error: %s", resp.Error))
				return
			}

			// Stream-end PDUs.
			if resp.Unsubscribe != nil || resp.Canceled != nil {
				return
			}

			// Skip state-enter / state-leave PDUs.
			if resp.StateEnter != nil || resp.StateLeave != nil {
				continue
			}

			paths := []string{}
			clockSpec := resp.Clock

			// Only run dedup when the PDU actually carried a files field.
			if resp.Files != nil {
				diffHashes := make(map[string]string, len(resp.Files))
				paths = make([]string, len(resp.Files))
				for i, f := range resp.Files {
					paths[i] = f.Name
					// content.sha1hex is either a hex string or an error object;
					// peek the first byte to distinguish without re-decoding.
					if len(f.Sha1Hex) > 2 && f.Sha1Hex[0] == '"' {
						diffHashes[f.Name] = string(f.Sha1Hex[1 : len(f.Sha1Hex)-1])
					} else {
						// No known sha1 or error calculating the sha1, use the
						// clock spec as a proxy for file content change
						diffHashes[f.Name] = clockSpec
					}
				}

				if maps.Equal(diffHashes, prevDiffHashes) {
					// Check for sequential duplicate change events from watchman and ignore them.
					// Watchman can sometimes send duplicate events for the same change depending
					// on the platform, filesystem and things such as which tool made the fs change.
					//
					// For example it has been reported that the coreutils `cp` command on macOS
					// can trigger duplicate events for the same file copy operation while other forms
					// of file copy do not.
					continue
				}

				prevDiffHashes = diffHashes
			}

			cs := ChangeSet{
				Paths:           paths,
				Root:            path.Join(resp.Root, w.watchedRelPath),
				ClockSpec:       clockSpec,
				IsFreshInstance: resp.IsFreshInstance,
			}
			if !yield(&cs, nil) {
				return
			}
		}
	}
}

func (w *WatchmanWatcher) StateEnter(name string) error {
	if err := w.send("state-enter", w.watchedRoot, name); err != nil {
		return fmt.Errorf("failed to send state-enter command: %w", err)
	}
	resp, err := w.recv()
	if err != nil {
		return fmt.Errorf("failed to receive state-enter command response: %w", err)
	}
	if resp.StateEnter == nil {
		return fmt.Errorf("unknown state-enter response: %+v", resp)
	}
	if *resp.StateEnter != name {
		return fmt.Errorf("failed to state-enter: %s != %s in %+v", name, *resp.StateEnter, resp)
	}
	return nil
}

func (w *WatchmanWatcher) StateLeave(name string) error {
	if err := w.send("state-leave", w.watchedRoot, name); err != nil {
		return fmt.Errorf("failed to send state-leave command: %w", err)
	}
	resp, err := w.recv()
	if err != nil {
		return fmt.Errorf("failed to receive state-leave command response: %w", err)
	}
	if resp.StateLeave == nil {
		return fmt.Errorf("unknown state-leave response: %+v", resp)
	}
	if *resp.StateLeave != name {
		return fmt.Errorf("failed to state-leave: %s != %s in %+v", name, *resp.StateLeave, resp)
	}
	return nil
}

// NewWatchmanWatcher creates a new WatchmanWatcher
func NewWatchman(workspaceDir string) *WatchmanWatcher {
	return &WatchmanWatcher{workspaceDir: workspaceDir}
}

var _ Watcher = (*WatchmanWatcher)(nil)
