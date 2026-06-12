package typescript

import (
	"fmt"
	"os"
	"path"
	"strings"
	"sync"

	BazelLog "github.com/aspect-build/aspect-gazelle/common/logger"
	node "github.com/aspect-build/aspect-gazelle/language/js/node"
	pnpm "github.com/aspect-build/aspect-gazelle/language/js/pnpm"
)

type workspacePath struct {
	root     string
	rel      string
	fileName string
}

type TsConfigMap struct {
	// `configFiles` is created during the gazelle configure phase which is single threaded so doesn't
	// require mutex projection. Just `configs` has concurrency considerations since it is lazy
	// loading on multiple threads in the generate phase.
	configFiles map[string]map[string]*workspacePath

	configs      map[string]*TsConfig
	configsMutex sync.RWMutex
	pnpmProjects *pnpm.PnpmProjectMap
}

type TsWorkspace struct {
	cm *TsConfigMap

	// hasPackageJson reports whether a dir contains a package.json; such dirs
	// are anchors for forwarding ts_config rules. The backing data must be
	// recorded during the configure phase (single threaded) and is only
	// queried during the generate phase, matching the `configFiles` invariant.
	hasPackageJson func(rel string) bool
}

func NewTsWorkspace(pnpmProjects *pnpm.PnpmProjectMap, hasPackageJson func(rel string) bool) *TsWorkspace {
	return &TsWorkspace{
		cm: &TsConfigMap{
			configFiles:  make(map[string]map[string]*workspacePath),
			configs:      make(map[string]*TsConfig),
			pnpmProjects: pnpmProjects,
			configsMutex: sync.RWMutex{},
		},
		hasPackageJson: hasPackageJson,
	}
}

func (tc *TsWorkspace) SetTsConfigFile(root, rel, groupName, fileName string) {
	if tc.cm.configFiles[rel] == nil {
		tc.cm.configFiles[rel] = make(map[string]*workspacePath)
	}

	if c := tc.cm.configFiles[rel][groupName]; c != nil {
		fmt.Fprintf(os.Stderr, "Duplicate tsconfig file %s: %s and %s\n", path.Join(rel, fileName), c.rel, c.fileName)
		return
	}

	BazelLog.Debugf("Declaring tsconfig file %s: %s", rel, fileName)

	tc.cm.configFiles[rel][groupName] = &workspacePath{
		root:     root,
		rel:      rel,
		fileName: fileName,
	}
}

func (tc *TsWorkspace) GetTsConfigFile(rel, groupName string) *TsConfig {
	// No file exists
	p := tc.cm.configFiles[rel][groupName]
	if p == nil {
		return nil
	}
	return tc.getTsConfigFromPath(p)
}

func (tc *TsWorkspace) getTsConfigFromPath(p *workspacePath) *TsConfig {
	filePath := path.Join(p.rel, p.fileName)

	// Fast path: cache hit under a read lock.
	tc.cm.configsMutex.RLock()
	c := tc.cm.configs[filePath]
	tc.cm.configsMutex.RUnlock()

	if c == nil {
		// Slow path: parseTsConfigJSONFile re-checks the cache under the write lock.
		tc.cm.configsMutex.Lock()
		defer tc.cm.configsMutex.Unlock()

		var err error
		if c, err = parseTsConfigJSONFile(tc.cm.configs, tc.tsConfigResolver, p.root, filePath); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to parse tsconfig file %s: %v\n", filePath, err)
			return nil
		}
	}

	if c == &InvalidTsconfig {
		return nil
	}
	return c
}

// A `TsConfigResolver` to resolve imports from *within* tsconfig files
// to real paths such as resolved the tsconfig `extends`.
func (tc *TsWorkspace) tsConfigResolver(dir, rel string) []string {
	possible := []string{}

	if isRelativePath(rel) {
		possible = append(possible, path.Join(dir, rel))
	}

	if p := tc.cm.pnpmProjects.GetProject(dir); p != nil {
		pkg, subFile := node.ParseImportPath(rel)
		if pkg != "" {
			localRef, found := p.GetLocalReference(pkg)
			if found {
				possible = append(possible, path.Join(localRef, subFile))
			}
		}
	}

	return possible
}

// FindConfig walks up from `dir` for `groupName`'s tsconfig, falling back to
// the default. Returns (anchorRel, configRel, config): the ts_config rule's
// dir (a closer forwarding rule if one exists), the tsconfig file's dir, and
// the parsed config (nil if none).
func (tc *TsWorkspace) FindConfig(dir, groupName string) (string, string, *TsConfig) {
	// The closest default in case no group config is found
	var defaultRel string
	var defaultConfig *TsConfig

	// Group-specific config, if found.
	var configRel string
	var config *TsConfig

	// Closest forwarding ts_config rule; substituted for the config dir in
	// returned labels so callers anchor here, not past us at the real tsconfig.
	packageJsonDirRel, foundPackageJsonDir := "", false

	for {
		if dir == "." {
			dir = ""
		}

		if groupName != "" {
			if c := tc.GetTsConfigFile(dir, groupName); c != nil {
				configRel, config = dir, c
				break
			}
		}

		// Record the default group config as a fallback in case no group config is found
		if defaultConfig == nil {
			if c := tc.GetTsConfigFile(dir, ""); c != nil {
				defaultRel, defaultConfig = dir, c
				if groupName == "" {
					break
				}
			}
		}

		// Record the closest package.json dir; validated against configRel below.
		if !foundPackageJsonDir && tc.hasPackageJson(dir) {
			packageJsonDirRel, foundPackageJsonDir = dir, true
		}

		if dir == "" {
			break
		}

		dir, _ = path.Split(dir)
		dir = strings.TrimSuffix(dir, "/")
	}

	// Fall back to the default if no group-specific config was found.
	if config == nil {
		configRel, config = defaultRel, defaultConfig
	}

	// Use the recorded package.json dir as the anchor only if it sits at or
	// below configRel (never above it).
	anchorRel := configRel
	if foundPackageJsonDir && config != nil &&
		(configRel == "" || packageJsonDirRel == configRel || strings.HasPrefix(packageJsonDirRel, configRel+"/")) {
		anchorRel = packageJsonDirRel
	}
	return anchorRel, configRel, config
}

func (tc *TsWorkspace) ExpandPaths(from, f, groupName string) []string {
	d, _ := path.Split(from)
	_, _, c := tc.FindConfig(d, groupName)
	if c == nil {
		return []string{}
	}

	return c.ExpandPaths(from, f)
}
