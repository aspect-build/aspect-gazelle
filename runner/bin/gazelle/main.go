package main

import (
	"log"
	"os"

	"github.com/aspect-build/aspect-gazelle/common/bazel"
	"github.com/aspect-build/aspect-gazelle/common/cache"
	"github.com/aspect-build/aspect-gazelle/runner"
	"github.com/aspect-build/aspect-gazelle/runner/pkg/ibp"
	"github.com/aspect-build/aspect-gazelle/runner/pkg/watchman"
	"github.com/bazelbuild/bazel-gazelle/config"
)

/**
 * A `gazelle_binary` replacement where languages can be toggled at runtime.
 *
 * Supports additional features such as incremental builds via the Incremental Build Protocol,
 * interactive terminal progress, tracing and more.
 */
func main() {
	log.SetPrefix("aspect-gazelle: ")
	log.SetFlags(0) // don't print timestamps

	wd := bazel.FindWorkspaceDirectory()

	cmd, mode, progress, ct, args := parseArgs(os.Args[1:])

	c := runner.New(wd, progress)

	// Add languages
	for _, lang := range envLanguages {
		c.AddLanguage(lang)
	}

	if watchSocket := os.Getenv(ibp.PROTOCOL_SOCKET_ENV); watchSocket != "" {
		err := c.Watch(watchSocket, cmd, mode, args)
		if err != nil {
			log.Fatalf("Error running gazelle watcher: %v", err)
		}
	} else {
		switch ct {
		case cacheDisk:
			cache.SetCacheFactory(func(c *config.Config) cache.Cache {
				return cache.NewDiskCache(cache.FilePath(c))
			})
		case cacheWatchman:
			cache.SetCacheFactory(watchman.NewWatchmanCache)
		}

		hasChanges, err := c.Generate(cmd, mode, args)
		if err != nil {
			log.Fatalf("Error running gazelle: %v", err)
		}

		// Exit with code 1 if changes exit and not auto-fixed
		// See:
		//	- https://github.com/bazel-contrib/bazel-gazelle/blob/v0.47.0/cmd/gazelle/main.go#L73-L74
		//  - https://github.com/bazel-contrib/bazel-gazelle/blob/v0.47.0/cmd/gazelle/diff.go#L106
		if hasChanges && mode != runner.Fix {
			os.Exit(1)
		}
	}
}
