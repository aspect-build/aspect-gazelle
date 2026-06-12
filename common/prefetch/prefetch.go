// Package prefetch lets gazelle languages start expensive per-file work
// (reading + parsing source files) ahead of the serial directory walk.
//
// A background traversal over walk.GetDirInfo — the public window into the
// directory cache the walker itself populates concurrently, so no extra I/O
// — announces each directory listing well before the serial visit reaches
// it. Registered per-language prefetchers turn those announcements into
// Prefetch calls, which schedule the work on the shared common worker pool.
// When GenerateRules later needs the result it calls LoadOrCompute with the
// same key and either takes the finished result, waits for the in-flight
// one, or — when prefetching never happened (prefetch disabled, differing
// per-directory config) — computes inline.
//
// Prefetching is strictly best-effort: every result is keyed by everything
// the computation depends on, so a missing or stale prefetch only costs an
// inline compute, never a wrong answer.
package prefetch

import (
	"flag"
	"os"
	"path"
	"sync"
	"sync/atomic"

	"github.com/aspect-build/aspect-gazelle/common"
	BazelLog "github.com/aspect-build/aspect-gazelle/common/logger"
	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/language"
	"github.com/bazelbuild/bazel-gazelle/rule"
	"github.com/bazelbuild/bazel-gazelle/walk"
)

const gazelleExtensionKey = "__aspect:prefetch"

// DirPrefetcher receives each directory the walker has listed: rel is the
// slash-separated path relative to the repo root ("" for the root),
// regularFiles are the directory's file names after exclusion/ignore
// filtering, and pkgRel is the nearest ancestor directory (possibly rel
// itself) holding a build file — the package gazelle will flatten this
// directory's files into, and therefore the directory GenerateRules will
// report these files relative to. Implementations are called concurrently
// on shared pool workers and must be thread-safe.
type DirPrefetcher = func(rel string, regularFiles []string, pkgRel string)

// Get returns the Coordinator for this run, or nil when prefetching is not
// configured. A nil Coordinator is valid: Prefetch is a no-op and
// LoadOrCompute computes inline.
func Get(c *config.Config) *Coordinator {
	if v, ok := c.Exts[gazelleExtensionKey]; ok {
		return v.(*Coordinator)
	}
	return nil
}

// NewConfigurer returns the config.Configurer wiring a per-run Coordinator
// into config.Exts. It must be ordered before the language configurers so
// languages can Register during their CheckFlags.
//
// walkAll reports whether this run visits the entire workspace. The
// background traversal always enumerates the whole tree, so it is only
// started for full runs; targeted runs (specific directories, watch deltas)
// fall back to inline computes.
func NewConfigurer(walkAll bool) config.Configurer {
	return &prefetchConfigurer{co: &Coordinator{}, walkAll: walkAll}
}

type dirEvent struct {
	rel          string
	regularFiles []string
	pkgRel       string
}

type Coordinator struct {
	mu          sync.Mutex
	prefetchers []DirPrefetcher
	// Every directory announced so far, replayed to late registrants: a
	// language may only become ready to prefetch partway into the walk (e.g.
	// once the repo-root configuration exists), well after the walker's
	// background traversal started announcing directories.
	dirLog   []dirEvent
	promises sync.Map // string -> *promise

	// Set when the run completes; stops the background traversal, which must
	// not touch the walker after the walk returns.
	stopped atomic.Bool

	// Effectiveness counters, logged when the run completes.
	prefetched atomic.Int64 // promises scheduled by Prefetch
	hits       atomic.Int64 // LoadOrCompute found a finished/running promise
	steals     atomic.Int64 // LoadOrCompute claimed a still-queued promise
	misses     atomic.Int64 // LoadOrCompute found no promise for the key
}

// Register adds a language prefetcher. Directories announced before
// registration are replayed to it, so languages may register late — e.g.
// during the repo-root Configure — without missing anything.
func (co *Coordinator) Register(p DirPrefetcher) {
	if co == nil {
		return
	}
	co.mu.Lock()
	co.prefetchers = append(co.prefetchers, p)
	backlog := co.dirLog[:len(co.dirLog):len(co.dirLog)]
	co.mu.Unlock()

	if len(backlog) > 0 {
		common.Submit(func() {
			for _, ev := range backlog {
				p(ev.rel, ev.regularFiles, ev.pkgRel)
			}
		})
	}
}

// dirLoaded records the directory and dispatches it to the registered
// prefetchers on the shared pool, keeping the traversal goroutine free of
// prefetch-filtering work.
func (co *Coordinator) dirLoaded(rel string, regularFiles []string, pkgRel string) {
	co.mu.Lock()
	co.dirLog = append(co.dirLog, dirEvent{rel: rel, regularFiles: regularFiles, pkgRel: pkgRel})
	prefetchers := co.prefetchers[:len(co.prefetchers):len(co.prefetchers)]
	co.mu.Unlock()

	if len(prefetchers) > 0 {
		common.Submit(func() {
			for _, p := range prefetchers {
				p(rel, regularFiles, pkgRel)
			}
		})
	}
}

// traverse announces every directory in the workspace to the registered
// prefetchers, parent-first, via walk.GetDirInfo — the public window into
// the directory cache the gazelle walker concurrently populates, so this
// adds no directory I/O of its own, it just follows the walker's own
// background enumeration.
func (co *Coordinator) traverse() {
	var visit func(rel, pkgRel string)
	visit = func(rel, pkgRel string) {
		if co.stopped.Load() {
			return
		}
		info, ok := co.dirInfo(rel)
		if !ok {
			return
		}
		if info.File != nil {
			// This directory is a package; gazelle reports descendant
			// non-package directories' files relative to it.
			pkgRel = rel
		}
		co.dirLoaded(rel, info.RegularFiles, pkgRel)
		for _, sub := range info.Subdirs {
			visit(path.Join(rel, sub), pkgRel)
		}
	}
	visit("", "")
}

// dirInfo wraps walk.GetDirInfo, which may only be called while the walk is
// running: it panics once Walk2 returns. The stopped flag (set when
// generation completes) closes most of that window; the recover contains the
// inherent race remainder, abandoning the traversal.
func (co *Coordinator) dirInfo(rel string) (info walk.DirInfo, ok bool) {
	defer func() {
		if r := recover(); r != nil {
			BazelLog.Infof("prefetch traversal stopped: %v", r)
			co.stopped.Store(true)
			ok = false
		}
	}()
	info, err := walk.GetDirInfo(rel)
	if err != nil {
		return info, false
	}
	return info, true
}

// promise is a unit of prefetched work. The claimed flag resolves the race
// between the queued pool task and a LoadOrCompute caller: whoever claims it
// computes; the queued task becomes a no-op if the consumer got there first
// (work stealing). This also guarantees pool workers only ever wait on
// computations that are actively running, never on queued ones — so awaiting
// a promise from a pool worker cannot deadlock.
type promise struct {
	claimed atomic.Bool
	done    chan struct{}
	result  any
	err     error
}

// compute claims and runs fn, returning false if already claimed.
func (p *promise) compute(fn func() (any, error)) bool {
	if !p.claimed.CompareAndSwap(false, true) {
		return false
	}
	p.result, p.err = fn()
	close(p.done)
	return true
}

// Prefetch schedules computeFn under key on the shared worker pool. Only the
// first Prefetch of a key schedules work; later calls are no-ops. The key
// must capture everything the result depends on.
func (co *Coordinator) Prefetch(key string, computeFn func() (any, error)) {
	if co == nil {
		return
	}
	p := &promise{done: make(chan struct{})}
	if _, loaded := co.promises.LoadOrStore(key, p); loaded {
		return
	}
	co.prefetched.Add(1)
	common.Submit(func() {
		p.compute(computeFn)
	})
}

// LoadOrCompute returns the result for key: the prefetched result if
// finished, awaiting it if in flight, stealing it if still queued, or
// computing inline when key was never prefetched (or co is nil).
func (co *Coordinator) LoadOrCompute(key string, computeFn func() (any, error)) (any, error) {
	if co == nil {
		return computeFn()
	}
	v, ok := co.promises.Load(key)
	if !ok {
		co.misses.Add(1)
		BazelLog.Debugf("prefetch miss: %s", key)
		return computeFn()
	}
	p := v.(*promise)
	if p.compute(computeFn) {
		co.steals.Add(1)
	} else {
		co.hits.Add(1)
		<-p.done
	}
	return p.result, p.err
}

var _ config.Configurer = (*prefetchConfigurer)(nil)
var _ language.FinishableLanguage = (*prefetchConfigurer)(nil)

type prefetchConfigurer struct {
	co        *Coordinator
	walkAll   bool
	startOnce sync.Once
}

func (pc *prefetchConfigurer) CheckFlags(fs *flag.FlagSet, c *config.Config) error {
	c.Exts[gazelleExtensionKey] = pc.co
	return nil
}

// Configure starts the background traversal at the repo root: by then the
// walk is running, so walk.GetDirInfo is available. Configurers run before
// the language configurers at the root, but the replayed dirLog means even
// languages that register later (e.g. orion at its own root Configure) see
// every directory.
func (pc *prefetchConfigurer) Configure(c *config.Config, rel string, f *rule.File) {
	if rel != "" || !pc.walkAll {
		return
	}
	// Escape hatch: disable the speculative parse-ahead while keeping the
	// LoadOrCompute plumbing (which then always computes inline).
	if os.Getenv("ASPECT_GAZELLE_PREFETCH") == "off" {
		return
	}
	pc.startOnce.Do(func() {
		go pc.co.traverse()
	})
}

// DoneGeneratingRules stops the traversal and drops the run's promises so
// retained parse results can be collected once generation completes (notably
// between watch cycles).
func (pc *prefetchConfigurer) DoneGeneratingRules() {
	pc.co.stopped.Store(true)
	BazelLog.Infof("prefetch: %d prefetched, consumer hits %d, steals %d, misses %d",
		pc.co.prefetched.Load(), pc.co.hits.Load(), pc.co.steals.Load(), pc.co.misses.Load())
	pc.co.promises.Clear()
	pc.co.mu.Lock()
	pc.co.dirLog = nil
	pc.co.mu.Unlock()
}

func (pc *prefetchConfigurer) RegisterFlags(fs *flag.FlagSet, cmd string, c *config.Config) {}
func (pc *prefetchConfigurer) KnownDirectives() []string                                    { return nil }
