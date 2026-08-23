package stareval

import (
	"fmt"
	"maps"
	"path"
	"path/filepath"
	"strings"

	"github.com/bazelbuild/bazel-gazelle/label"
	"go.starlark.net/lib/json"
	"go.starlark.net/starlark"
	"go.starlark.net/syntax"

	stdlib "github.com/aspect-build/aspect-gazelle/language/orion/starlark/stdlib"
)

// The signature for a starlark module loader (see starlark.Thread.Load)
type moduleLoader = func(thread *starlark.Thread, module string) (starlark.StringDict, error)

// Thread-local holding the path of the file currently being evaluated, so that
// load() can resolve paths relative to the loading file rather than only
// relative to the workspace root.
const currentFileKey = "orion:current_file"

func currentFile(thread *starlark.Thread) string {
	if f, ok := thread.Local(currentFileKey).(string); ok {
		return f
	}
	return ""
}

// Remain simple and strict like bazel starlark.
var opts = &syntax.FileOptions{
	TopLevelControl: true,
	GlobalReassign:  false,
}

// Copy of go.starlark.net/repl.MakeLoadOptions with the following changes:
// * Add and passthru ExecFileOptions `src interface{}, predeclared starlark.StringDict`
//
// See https://github.com/google/starlark-go/blob/0d3f41d403af5d6607cdf241f12b7e0572f2cb58/repl/repl.go#L171-L200
func makeLoadOptions(opts *syntax.FileOptions, predeclared starlark.StringDict) moduleLoader {
	type entry struct {
		globals starlark.StringDict
		err     error
	}

	var cache = make(map[string]*entry)

	return func(thread *starlark.Thread, module string) (starlark.StringDict, error) {
		e, ok := cache[module]
		if e == nil {
			if ok {
				// request for package whose loading is in progress
				return nil, fmt.Errorf("cycle in load graph")
			}

			// Add a placeholder to indicate "load in progress".
			cache[module] = nil

			// Load it.
			thread := &starlark.Thread{Name: "exec " + module, Load: thread.Load}
			thread.SetLocal(currentFileKey, module)
			globals, err := starlark.ExecFileOptions(opts, thread, module, nil, predeclared)
			e = &entry{globals, err}

			// Update the cache.
			cache[module] = e
		}
		return e.globals, e.err
	}
}

// Wrap a `moduleLoader` and add support for load()ing similar to bazel rulesets.
func createRepoLoader(rootDir string, loader moduleLoader) moduleLoader {
	return func(thread *starlark.Thread, module string) (starlark.StringDict, error) {
		// A "./" prefix is relative to the file doing the load. This lets a set
		// of plugin files load each other regardless of where they are
		// materialized, which a workspace-root-relative path cannot express for
		// files outside the workspace (eg an external repo). Reach a parent
		// directory with "./../".
		//
		// Bare and "../" prefixed paths keep resolving from the workspace root:
		// "../" already works there as an escape out of the workspace, and
		// reinterpreting it would break plugins relying on that.
		if strings.HasPrefix(module, "./") {
			from := currentFile(thread)
			if from == "" {
				return nil, fmt.Errorf("relative load() outside of a file: %s", module)
			}
			return loader(thread, path.Join(path.Dir(from), module))
		}

		moduleLabel, err := label.Parse(module)
		if err != nil {
			return nil, fmt.Errorf("invalid load() label: %s", module)
		}

		if moduleLabel.Repo != "" {
			// Without an explicit target, label.Parse infers Name from the last
			// segment of Pkg, so joining both would repeat it.
			repoPath := moduleLabel.Pkg
			if strings.Contains(module, ":") {
				repoPath = path.Join(moduleLabel.Pkg, moduleLabel.Name)
			}

			modulePath, err := runfilesPath(currentFile(thread), moduleLabel.Repo, repoPath)
			if err != nil {
				return nil, err
			}
			return loader(thread, modulePath)
		}

		modulePath := path.Join(rootDir, moduleLabel.Pkg, moduleLabel.Name)

		return loader(thread, modulePath)
	}
}

func threadPrint(t *starlark.Thread, msg string) {
	// TODO: stdout? log?
	fmt.Printf("%s: %s\n", t.Name, msg)
}

func Eval(rootDir, starpath string, libs starlark.StringDict, locals map[string]any) (starlark.StringDict, error) {
	// load() resolution is slash-only; normalize OS-separator inputs (eg a Windows RUNFILES_DIR)
	rootDir = filepath.ToSlash(rootDir)
	starpath = filepath.ToSlash(starpath)

	// Predeclared libs in addition to the go.starlark.net/starlark standard library:
	// * https://github.com/google/starlark-go/blob/f86470692795f8abcf9f837a3c53cf031c5a3d7e/starlark/library.go#L36-L73
	// * https://github.com/google/starlark-go/blob/f86470692795f8abcf9f837a3c53cf031c5a3d7e/cmd/starlark/starlark.go#L96-L100
	predeclared := starlark.StringDict{
		"path": stdlib.Path,
		"json": json.Module,
	}

	maps.Copy(predeclared, libs)

	loader := makeLoadOptions(opts, predeclared)
	loader = createRepoLoader(rootDir, loader)

	thread := starlark.Thread{
		Name:  "AspectConfigure",
		Load:  loader,
		Print: threadPrint,
	}
	for localName, local := range locals {
		thread.SetLocal(localName, local)
	}

	entrypoint := path.Join(rootDir, starpath)
	thread.SetLocal(currentFileKey, entrypoint)

	return starlark.ExecFileOptions(opts, &thread, entrypoint, nil, predeclared)
}
