package typescript

import (
	"fmt"
	"path"
	"slices"
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
	configFiles  map[string]map[string]*workspacePath
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
			configFiles:  make(map[string]map[string]*workspacePath),
			configs:      make(map[string]*TsConfig),
			pnpmProjects: pnpmProjects,
			configsMutex: sync.RWMutex{},
		},
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

type TsConfigWithGroup struct {
	Config    *TsConfig
	GroupName string
}

func (tc *TsWorkspace) GetAllTsConfigFiles(rel string) []TsConfigWithGroup {
	inner := tc.cm.configFiles[rel]
	configs := make([]TsConfigWithGroup, 0, len(inner))
	for groupName, p := range inner {
		if c := tc.getTsConfigFromPath(p); c != nil {
			configs = append(configs, TsConfigWithGroup{Config: c, GroupName: groupName})
		}
	}
	// Required for deterministic output order of generated targets
	slices.SortFunc(configs, func(a, b TsConfigWithGroup) int {
		return strings.Compare(a.Config.ConfigName, b.Config.ConfigName)
	})
	return configs
}

func (tc *TsWorkspace) getTsConfigFromPath(p *workspacePath) *TsConfig {
	// Lock the configs mutex
	tc.cm.configsMutex.Lock()
	defer tc.cm.configsMutex.Unlock()

	filePath := path.Join(p.rel, p.fileName)

	// Check for previously parsed
	if c := tc.cm.configs[filePath]; c != nil {
		if c == &InvalidTsconfig {
			return nil
		}
		return c
	}

	c, err := parseTsConfigJSONFile(tc.cm.configs, tc.tsConfigResolver, p.root, filePath)
	if err != nil {
		fmt.Printf("Failed to parse tsconfig file %s: %v\n", filePath, err)
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

func (tc *TsWorkspace) FindConfig(dir, groupName string) (string, *TsConfig) {
	for {
		if dir == "." {
			dir = ""
		}

		if c := tc.GetTsConfigFile(dir, groupName); c != nil {
			return dir, c
		}

		// Fall back to the default group config if a group-specific one isn't found.
		if c := tc.GetTsConfigFile(dir, ""); c != nil {
			return dir, c
		}

		if dir == "" {
			break
		}

		dir, _ = path.Split(dir)
		dir = strings.TrimSuffix(dir, "/")
	}

	return "", nil
}

func (tc *TsWorkspace) ExpandPaths(from, f, groupName string) []string {
	d, _ := path.Split(from)
	_, c := tc.FindConfig(d, groupName)
	if c == nil {
		return []string{}
	}

	return c.ExpandPaths(from, f)
}
