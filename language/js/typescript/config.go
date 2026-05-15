package typescript

import (
	"fmt"
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

	// Dirs with a package.json; anchors for forwarding ts_config rules.
	// Same configure-phase-write / generate-phase-read invariant as `configFiles`.
	packageJsonDirs map[string]bool

	configs      map[string]*TsConfig
	configsMutex sync.RWMutex
	pnpmProjects *pnpm.PnpmProjectMap
}

type TsWorkspace struct {
	cm *TsConfigMap
}

func NewTsWorkspace(pnpmProjects *pnpm.PnpmProjectMap) *TsWorkspace {
	return &TsWorkspace{
		cm: &TsConfigMap{
			configFiles:     make(map[string]map[string]*workspacePath),
			packageJsonDirs: make(map[string]bool),
			configs:         make(map[string]*TsConfig),
			pnpmProjects:    pnpmProjects,
			configsMutex:    sync.RWMutex{},
		},
	}
}

// RegisterPackageJsonDir records a dir containing a package.json so FindConfig
// anchors at the local forwarding ts_config rather than the ancestor tsconfig.
func (tc *TsWorkspace) RegisterPackageJsonDir(rel string) {
	tc.cm.packageJsonDirs[rel] = true
}

// ClosestAncestorPackageJsonDir returns the closest strictly-ancestor dir of
// `dir` registered as a package.json dir, or "", false.
func (tc *TsWorkspace) ClosestAncestorPackageJsonDir(dir string) (string, bool) {
	for {
		base, _ := path.Split(dir)
		dir = strings.TrimSuffix(base, "/")
		if tc.cm.packageJsonDirs[dir] {
			return dir, true
		}
		if dir == "" {
			return "", false
		}
	}
}

func (tc *TsWorkspace) SetTsConfigFile(root, rel, groupName, fileName string) {
	if tc.cm.configFiles[rel] == nil {
		tc.cm.configFiles[rel] = make(map[string]*workspacePath)
	}

	if c := tc.cm.configFiles[rel][groupName]; c != nil {
		fmt.Printf("Duplicate tsconfig file %s: %s and %s", path.Join(rel, fileName), c.rel, c.fileName)
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
			fmt.Printf("Failed to parse tsconfig file %s: %v\n", filePath, err)
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

		if !foundPackageJsonDir && tc.cm.packageJsonDirs[dir] && defaultConfig == nil {
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

	anchorRel := configRel
	if foundPackageJsonDir && config != nil {
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
