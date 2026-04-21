/*
 * Copyright 2022 Aspect Build Systems, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package typescript

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	BazelLog "github.com/aspect-build/aspect-gazelle/common/logger"
	"github.com/goexlib/jsonc"
)

type tsCompilerOptionsJSON struct {
	AllowJs              *bool                `json:"allowJs"`
	Composite            *bool                `json:"composite"`
	Declaration          *bool                `json:"declaration"`
	DeclarationDir       *string              `json:"declarationDir"`
	DeclarationMap       *bool                `json:"declarationMap"`
	DeclarationOnly      *bool                `json:"emitDeclarationOnly"`
	Incremental          *bool                `json:"incremental"`
	IsolatedDeclarations *bool                `json:"isolatedDeclarations"`
	TsBuildInfoFile      *string              `json:"tsBuildInfoFile"`
	SourceMap            *bool                `json:"sourceMap"`
	ResolveJsonModule    *bool                `json:"resolveJsonModule"`
	NoEmit               *bool                `json:"noEmit"`
	OutDir               *string              `json:"outDir"`
	RootDir              *string              `json:"rootDir"`
	RootDirs             *[]string            `json:"rootDirs"`
	BaseUrl              *string              `json:"baseUrl"`
	Paths                *map[string][]string `json:"paths"`
	Types                *[]string            `json:"types"`
	JSX                  *TsConfigJsxType     `json:"jsx"`
	ImportHelpers        *bool                `json:"importHelpers"`
}

type tsReferenceJSON struct {
	Path string `json:"path"`
}

type tsExtendsJSON []string

func (s *tsExtendsJSON) UnmarshalJSON(data []byte) error {
	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		*s = []string{single}
		return nil
	}
	var multiple []string
	if err := json.Unmarshal(data, &multiple); err != nil {
		return fmt.Errorf("expected string or []string: %w", err)
	}
	*s = multiple
	return nil
}

type tsConfigJSON struct {
	Extends         tsExtendsJSON         `json:"extends"`
	CompilerOptions tsCompilerOptionsJSON `json:"compilerOptions"`
	References      *[]tsReferenceJSON    `json:"references"`
}

type TsConfigResolver = func(dir, conf string) []string

// TsConfig JSX options: https://www.typescriptlang.org/tsconfig/#jsx
type TsConfigJsxType string

const (
	JsxNone        TsConfigJsxType = "none"
	JsxPreserve    TsConfigJsxType = "preserve"
	JsxReact       TsConfigJsxType = "react"
	JsxReactJsx    TsConfigJsxType = "react-jsx"
	JsxReactJsxDev TsConfigJsxType = "react-jsxdev"
	JsxReactNative TsConfigJsxType = "react-native"
)

func (j TsConfigJsxType) IsReact() bool {
	s := string(j)
	return s == "react" || strings.HasPrefix(s, "react-")
}

func expandConfigDirPath(value string) string {
	return path.Clean(strings.ReplaceAll(value, "${configDir}", "."))
}

func expandConfigDirFile(value string) string {
	if value == "" {
		return value
	}
	return path.Clean(strings.ReplaceAll(value, "${configDir}", "."))
}

type TsConfig struct {
	// Directory of the tsconfig file
	ConfigDir string

	// Name of the tsconfig file relative to ConfigDir
	ConfigName string

	AllowJs           *bool
	ResolveJsonModule *bool
	Composite         *bool
	Declaration       *bool
	DeclarationDir    *string
	DeclarationMap    *bool
	DeclarationOnly   *bool
	Incremental       *bool
	TsBuildInfoFile   string
	SourceMap         *bool
	NoEmit            *bool
	OutDir            string
	RootDir           string
	BaseUrl           string

	VirtualRootDirs []string

	Paths *TsConfigPaths

	ImportHelpers bool

	IsolatedDeclarations *bool

	// How jsx/tsx files are handled
	Jsx TsConfigJsxType

	// References to other tsconfig or packages that must be resolved.
	Types   []string
	Extends []string

	// TODO: drop references? Not supported by rules_ts?
	References []string

	// Which defaulted-string fields were explicitly set (for mergeBaseConfigs).
	set tsConfigExplicit
}

// Tracks which fields were explicitly set in JSON (vs defaulted / inherited-default), for mergeBaseConfigs.
type tsConfigExplicit struct {
	rootDir         bool
	outDir          bool
	baseUrl         bool
	tsBuildInfoFile bool
	importHelpers   bool
	paths           bool
}

type TsConfigPaths struct {
	Rel string
	Map map[string][]string
}

var DefaultConfigPaths = TsConfigPaths{
	Rel: ".",
	Map: map[string][]string{},
}

var InvalidTsconfig = TsConfig{
	Paths: &DefaultConfigPaths,
	Jsx:   JsxNone,
}

func isRelativePath(p string) bool {
	if p == "" || p[0] == '/' {
		return false
	}

	return p[0] == '.' && (p == "." || p == ".." || strings.HasPrefix(p, "./") || strings.HasPrefix(p, "../"))
}

// mergeBaseConfigs merges two parsed TsConfigs (right overrides left), rebasing paths to currentConfigDir.
func mergeBaseConfigs(left, right *TsConfig, currentConfigDir string) *TsConfig {
	// Compute relative paths from current config to each base config
	leftRel, leftErr := filepath.Rel(currentConfigDir, left.ConfigDir)
	if leftErr != nil {
		leftRel = "."
	}
	rightRel, rightErr := filepath.Rel(currentConfigDir, right.ConfigDir)
	if rightErr != nil {
		rightRel = "."
	}

	// ConfigDir = currentConfigDir so chained merges don't re-rebase paths.
	merged := &TsConfig{
		ConfigDir:  currentConfigDir,
		ConfigName: left.ConfigName,
	}

	// For nullable boolean fields: right overrides left
	merged.AllowJs = right.AllowJs
	if merged.AllowJs == nil {
		merged.AllowJs = left.AllowJs
	}

	merged.Composite = right.Composite
	if merged.Composite == nil {
		merged.Composite = left.Composite
	}

	merged.Declaration = right.Declaration
	if merged.Declaration == nil {
		merged.Declaration = left.Declaration
	}

	merged.DeclarationMap = right.DeclarationMap
	if merged.DeclarationMap == nil {
		merged.DeclarationMap = left.DeclarationMap
	}

	merged.DeclarationOnly = right.DeclarationOnly
	if merged.DeclarationOnly == nil {
		merged.DeclarationOnly = left.DeclarationOnly
	}

	merged.Incremental = right.Incremental
	if merged.Incremental == nil {
		merged.Incremental = left.Incremental
	}

	merged.IsolatedDeclarations = right.IsolatedDeclarations
	if merged.IsolatedDeclarations == nil {
		merged.IsolatedDeclarations = left.IsolatedDeclarations
	}

	merged.NoEmit = right.NoEmit
	if merged.NoEmit == nil {
		merged.NoEmit = left.NoEmit
	}

	merged.SourceMap = right.SourceMap
	if merged.SourceMap == nil {
		merged.SourceMap = left.SourceMap
	}

	merged.ResolveJsonModule = right.ResolveJsonModule
	if merged.ResolveJsonModule == nil {
		merged.ResolveJsonModule = left.ResolveJsonModule
	}

	// For string fields: right overrides left if not default
	merged.DeclarationDir = right.DeclarationDir
	if merged.DeclarationDir == nil {
		merged.DeclarationDir = left.DeclarationDir
	}

	// For string-with-default-"." fields the sidecar `set` flag tells us whether
	// the value was explicitly set in JSON, so an explicit "." can override an
	// inherited non-"." value.
	if right.set.outDir {
		merged.OutDir = right.OutDir
		merged.set.outDir = true
	} else {
		merged.OutDir = left.OutDir
		merged.set.outDir = left.set.outDir
	}
	if right.set.rootDir {
		merged.RootDir = right.RootDir
		merged.set.rootDir = true
	} else {
		merged.RootDir = left.RootDir
		merged.set.rootDir = left.set.rootDir
	}
	if right.set.baseUrl {
		merged.BaseUrl = right.BaseUrl
		merged.set.baseUrl = true
	} else {
		merged.BaseUrl = left.BaseUrl
		merged.set.baseUrl = left.set.baseUrl
	}
	if right.set.tsBuildInfoFile {
		merged.TsBuildInfoFile = right.TsBuildInfoFile
		merged.set.tsBuildInfoFile = true
	} else {
		merged.TsBuildInfoFile = left.TsBuildInfoFile
		merged.set.tsBuildInfoFile = left.set.tsBuildInfoFile
	}

	// Paths: right replaces left only when explicitly set. Can't use pointer
	// identity with &DefaultConfigPaths because inherited paths get
	// materialized into a fresh struct during parse.
	if right.set.paths {
		merged.Paths = &TsConfigPaths{
			Rel: path.Join(rightRel, right.Paths.Rel),
			Map: right.Paths.Map,
		}
		merged.set.paths = true
	} else if left.set.paths {
		merged.Paths = &TsConfigPaths{
			Rel: path.Join(leftRel, left.Paths.Rel),
			Map: left.Paths.Map,
		}
		merged.set.paths = true
	} else {
		merged.Paths = &DefaultConfigPaths
	}

	// VirtualRootDirs: right replaces left when explicitly set (non-nil).
	if right.VirtualRootDirs != nil {
		merged.VirtualRootDirs = make([]string, len(right.VirtualRootDirs))
		for i, d := range right.VirtualRootDirs {
			merged.VirtualRootDirs[i] = path.Join(rightRel, d)
		}
	} else if left.VirtualRootDirs != nil {
		merged.VirtualRootDirs = make([]string, len(left.VirtualRootDirs))
		for i, d := range left.VirtualRootDirs {
			merged.VirtualRootDirs[i] = path.Join(leftRel, d)
		}
	}

	// ImportHelpers: right wins when explicitly set, else inherit left.
	if right.set.importHelpers {
		merged.ImportHelpers = right.ImportHelpers
		merged.set.importHelpers = true
	} else {
		merged.ImportHelpers = left.ImportHelpers
		merged.set.importHelpers = left.set.importHelpers
	}

	// Jsx: right overrides left if set
	merged.Jsx = right.Jsx
	if merged.Jsx == JsxNone {
		merged.Jsx = left.Jsx
	}

	return merged
}

// Load a tsconfig.json file and return the compilerOptions config with
// recursive protected via a parsed map that is passed in
func parseTsConfigJSONFile(parsed map[string]*TsConfig, resolver TsConfigResolver, root, tsconfig string) (*TsConfig, error) {
	existing := parsed[tsconfig]

	// Existing pointing to `InvalidTsconfig` implies recursion
	if existing == &InvalidTsconfig {
		BazelLog.Warnf("Recursive tsconfig file extension: %q", tsconfig)
		return nil, nil
	}

	// Already parsed and cached
	if existing != nil {
		return existing, nil
	}

	// Start with invalid to prevent recursing into the same file
	parsed[tsconfig] = &InvalidTsconfig

	tsconfigFile, err := os.OpenFile(path.Join(root, tsconfig), os.O_RDONLY, os.FileMode(os.O_RDONLY))
	if err != nil {
		return nil, err
	}
	defer tsconfigFile.Close()

	config, err := parseTsConfigJSON(parsed, resolver, root, tsconfig, tsconfigFile)
	if config != nil {
		BazelLog.Debugf("Parsed tsconfig file %s", tsconfig)

		parsed[tsconfig] = config
	}
	return config, err
}

func parseTsConfigJSON(parsed map[string]*TsConfig, resolver TsConfigResolver, root, tsconfig string, tsconfigReader io.Reader) (*TsConfig, error) {
	tsconfigData, err := io.ReadAll(tsconfigReader)
	if err != nil {
		return nil, err
	}

	var c tsConfigJSON
	if err := jsonc.Unmarshal(tsconfigData, &c); err != nil {
		return nil, err
	}

	configDir := path.Dir(tsconfig)
	configName := path.Base(tsconfig)

	var baseConfig *TsConfig
	var extends []string
	var bases []*TsConfig
	// Resolve each extended config; pass the raw path to the resolver since
	// path.Clean strips the "./" it uses to distinguish relative vs package imports.
	for _, ext := range c.Extends {
		if ext == "" {
			continue
		}
		extends = append(extends, path.Clean(ext))

		for _, potential := range resolver(configDir, ext) {
			base, err := parseTsConfigJSONFile(parsed, resolver, root, potential)
			if err != nil {
				BazelLog.Warnf("Failed to load base tsconfig file %q from %q: %v", ext, tsconfig, err)
			} else if base != nil {
				bases = append(bases, base)
				break
			}
		}
	}
	if len(bases) > 0 {
		baseConfig = bases[0]
		for i := 1; i < len(bases); i++ {
			baseConfig = mergeBaseConfigs(baseConfig, bases[i], configDir)
		}
	}

	var types []string
	if c.CompilerOptions.Types != nil && len(*c.CompilerOptions.Types) > 0 {
		types = *c.CompilerOptions.Types
	}

	var references []string
	if c.References != nil && len(*c.References) > 0 {
		references = make([]string, 0, len(*c.References))

		for _, r := range *c.References {
			if r.Path != "" {
				references = append(references, path.Join(configDir, r.Path))
			}
		}
	}

	var baseConfigRel = "."
	if baseConfig != nil {
		rel, relErr := filepath.Rel(configDir, baseConfig.ConfigDir)
		if relErr != nil {
			BazelLog.Warnf("Failed to resolve relative path from %s to %s: %v", configDir, baseConfig.ConfigDir, relErr)
		} else {
			baseConfigRel = rel
		}
	}

	var allowJs *bool
	if c.CompilerOptions.AllowJs != nil {
		allowJs = c.CompilerOptions.AllowJs
	} else if baseConfig != nil {
		allowJs = baseConfig.AllowJs
	}

	var composite *bool
	if c.CompilerOptions.Composite != nil {
		composite = c.CompilerOptions.Composite
	} else if baseConfig != nil {
		composite = baseConfig.Composite
	}

	var declaration *bool
	if c.CompilerOptions.Declaration != nil {
		declaration = c.CompilerOptions.Declaration
	} else if baseConfig != nil {
		declaration = baseConfig.Declaration
	}

	var declarationMap *bool
	if c.CompilerOptions.DeclarationMap != nil {
		declarationMap = c.CompilerOptions.DeclarationMap
	} else if baseConfig != nil {
		declarationMap = baseConfig.DeclarationMap
	}

	var declarationOnly *bool
	if c.CompilerOptions.DeclarationOnly != nil {
		declarationOnly = c.CompilerOptions.DeclarationOnly
	} else if baseConfig != nil {
		declarationOnly = baseConfig.DeclarationOnly
	}

	var incremental *bool
	if c.CompilerOptions.Incremental != nil {
		incremental = c.CompilerOptions.Incremental
	} else if baseConfig != nil {
		incremental = baseConfig.Incremental
	}

	var isolatedDeclarations *bool
	if c.CompilerOptions.IsolatedDeclarations != nil {
		isolatedDeclarations = c.CompilerOptions.IsolatedDeclarations
	} else if baseConfig != nil {
		isolatedDeclarations = baseConfig.IsolatedDeclarations
	}

	var noEmit *bool
	if c.CompilerOptions.NoEmit != nil {
		noEmit = c.CompilerOptions.NoEmit
	} else if baseConfig != nil {
		noEmit = baseConfig.NoEmit
	}

	var tsBuildInfoFile string
	var tsBuildInfoFileSet bool
	if c.CompilerOptions.TsBuildInfoFile != nil {
		tsBuildInfoFile = expandConfigDirFile(*c.CompilerOptions.TsBuildInfoFile)
		tsBuildInfoFileSet = true
	} else if baseConfig != nil {
		tsBuildInfoFile = baseConfig.TsBuildInfoFile
		tsBuildInfoFileSet = baseConfig.set.tsBuildInfoFile
	}

	var sourceMap *bool
	if c.CompilerOptions.SourceMap != nil {
		sourceMap = c.CompilerOptions.SourceMap
	} else if baseConfig != nil {
		sourceMap = baseConfig.SourceMap
	}

	var resolveJsonModule *bool
	if c.CompilerOptions.ResolveJsonModule != nil {
		resolveJsonModule = c.CompilerOptions.ResolveJsonModule
	} else if baseConfig != nil {
		resolveJsonModule = baseConfig.ResolveJsonModule
	}

	var RootDir string
	var rootDirSet bool
	if c.CompilerOptions.RootDir != nil {
		RootDir = expandConfigDirPath(*c.CompilerOptions.RootDir)
		rootDirSet = true
	} else if baseConfig != nil {
		RootDir = baseConfig.RootDir
		rootDirSet = baseConfig.set.rootDir
	} else {
		RootDir = "."
	}

	var OutDir string
	var outDirSet bool
	if c.CompilerOptions.OutDir != nil {
		OutDir = expandConfigDirPath(*c.CompilerOptions.OutDir)
		outDirSet = true
	} else if baseConfig != nil {
		OutDir = baseConfig.OutDir
		outDirSet = baseConfig.set.outDir
	} else {
		OutDir = "."
	}

	var declarationDir *string
	if c.CompilerOptions.DeclarationDir != nil {
		expanded := expandConfigDirPath(*c.CompilerOptions.DeclarationDir)
		declarationDir = &expanded
	} else if baseConfig != nil {
		declarationDir = baseConfig.DeclarationDir
	}

	var BaseUrl string
	var baseUrlSet bool
	if c.CompilerOptions.BaseUrl != nil {
		BaseUrl = expandConfigDirPath(*c.CompilerOptions.BaseUrl)
		baseUrlSet = true
	} else {
		BaseUrl = "."
	}

	var Paths *TsConfigPaths
	var pathsSet bool
	if c.CompilerOptions.Paths != nil {
		Paths = &TsConfigPaths{
			Rel: BaseUrl,
			Map: *c.CompilerOptions.Paths,
		}
		pathsSet = true
	} else if baseConfig != nil {
		Paths = &TsConfigPaths{
			Rel: path.Join(baseConfigRel, baseConfig.Paths.Rel),
			Map: baseConfig.Paths.Map,
		}
		pathsSet = baseConfig.set.paths
	} else {
		Paths = &DefaultConfigPaths
	}

	// rootDirs: nil = unset, non-nil (possibly empty) = explicitly set.
	var VirtualRootDirs []string
	if c.CompilerOptions.RootDirs != nil {
		VirtualRootDirs = make([]string, 0, len(*c.CompilerOptions.RootDirs))
		for _, d := range *c.CompilerOptions.RootDirs {
			VirtualRootDirs = append(VirtualRootDirs, path.Clean(d))
		}
	} else if baseConfig != nil && baseConfig.VirtualRootDirs != nil {
		VirtualRootDirs = make([]string, 0, len(baseConfig.VirtualRootDirs))
		for _, d := range baseConfig.VirtualRootDirs {
			VirtualRootDirs = append(VirtualRootDirs, path.Join(baseConfigRel, d))
		}
	}

	var importHelpers bool
	var importHelpersSet bool
	if c.CompilerOptions.ImportHelpers != nil {
		importHelpers = *c.CompilerOptions.ImportHelpers
		importHelpersSet = true
	} else if baseConfig != nil {
		importHelpers = baseConfig.ImportHelpers
		importHelpersSet = baseConfig.set.importHelpers
	}

	var jsx = JsxNone
	if c.CompilerOptions.JSX != nil {
		jsx = *c.CompilerOptions.JSX
	} else if baseConfig != nil {
		jsx = baseConfig.Jsx
	}

	config := TsConfig{
		ConfigDir:            configDir,
		ConfigName:           configName,
		AllowJs:              allowJs,
		Composite:            composite,
		Declaration:          declaration,
		DeclarationDir:       declarationDir,
		DeclarationMap:       declarationMap,
		DeclarationOnly:      declarationOnly,
		Incremental:          incremental,
		IsolatedDeclarations: isolatedDeclarations,
		TsBuildInfoFile:      tsBuildInfoFile,
		SourceMap:            sourceMap,
		ResolveJsonModule:    resolveJsonModule,
		NoEmit:               noEmit,
		OutDir:               OutDir,
		RootDir:              RootDir,
		BaseUrl:              BaseUrl,
		Paths:                Paths,
		VirtualRootDirs:      VirtualRootDirs,
		Extends:              extends,
		ImportHelpers:        importHelpers,
		Jsx:                  jsx,
		Types:                types,
		References:           references,
		set: tsConfigExplicit{
			rootDir:         rootDirSet,
			outDir:          outDirSet,
			baseUrl:         baseUrlSet,
			tsBuildInfoFile: tsBuildInfoFileSet,
			importHelpers:   importHelpersSet,
			paths:           pathsSet,
		},
	}

	return &config, nil
}

// Expands the path from the project base to the active tsconfig.json file
func (c TsConfig) expandBaseUrl(importPath string) string {
	if path.IsAbs(importPath) {
		return cleanJoin3("", c.BaseUrl, importPath)
	}
	return cleanJoin3(c.ConfigDir, c.BaseUrl, importPath)
}

// Expands the paths-mapped path relative to the tsconfig.json file
func (c TsConfig) expandPathsMatch(importPath string) string {
	if path.IsAbs(importPath) {
		return cleanJoin3("", c.Paths.Rel, importPath)
	}
	return cleanJoin3(c.ConfigDir, c.Paths.Rel, importPath)
}

// Expand the given path to all possible mapped paths for this config, in priority order.
//
// Path matching algorithm based on ESBuild implementation
// Inspired by: https://github.com/evanw/esbuild/blob/deb93e92267a96575a6e434ff18421f4ef0605e4/internal/resolver/resolver.go#L1831-L1945
func (c TsConfig) ExpandPaths(from, p string) []string {
	pathMap := c.Paths.Map
	possible := []string{}

	// Check for exact 'paths' matches first
	if exact := pathMap[p]; len(exact) > 0 {
		if BazelLog.IsTraceEnabled() {
			BazelLog.Tracef("TsConfig.paths exact matches for %q: %v", p, exact)
		}

		for _, m := range exact {
			possible = append(possible, c.expandPathsMatch(m))
		}
	}

	// Check for pattern matches next
	possibleMatches := make(matchArray, 0)
	for key, originalPaths := range pathMap {
		if before, after, ok := strings.Cut(key, "*"); ok {
			prefix, suffix := before, after

			if strings.HasPrefix(p, prefix) && strings.HasSuffix(p, suffix) {
				possibleMatches = append(possibleMatches, match{
					prefix:        prefix,
					suffix:        suffix,
					originalPaths: originalPaths,
				})
			}
		}
	}

	if len(possibleMatches) > 0 {
		// Sort the 'paths' pattern matches by priority
		sort.Sort(possibleMatches)

		if BazelLog.IsTraceEnabled() {
			BazelLog.Tracef("TsConfig.paths glob matches for %q: %v", p, possibleMatches)
		}

		// Expand and add the pattern matches
		for _, m := range possibleMatches {
			for _, originalPath := range m.originalPaths {
				// Swap out the "*" in the original path for whatever the "*" matched
				matchedText := p[len(m.prefix) : len(p)-len(m.suffix)]
				mappedPath := strings.Replace(originalPath, "*", matchedText, 1)

				possible = append(possible, c.expandPathsMatch(mappedPath))
			}
		}
	}

	// Expand paths from baseUrl
	// Must not to be absolute or relative to be expanded
	// https://www.typescriptlang.org/tsconfig#baseUrl
	if !isRelativePath(p) {
		baseUrlPath := c.expandBaseUrl(p)

		if BazelLog.IsTraceEnabled() {
			BazelLog.Tracef("TsConfig.baseUrl match for %q: %v", p, baseUrlPath)
		}

		possible = append(possible, baseUrlPath)
	}

	// Add 'rootDirs' as alternate directories for relative imports
	// https://www.typescriptlang.org/tsconfig#rootDirs
	for _, v := range c.VirtualRootDirs {
		possible = append(possible, cleanJoin3(v, "", p))
	}

	return possible
}

// Equivalent to path.Join(a, b, c) but uses minimal string concatenation and
// a single path.Clean() to append 1-3 path segments.
func cleanJoin3(a, b, c string) string {
	aEmpty := a == "" || a == "."
	bEmpty := b == "" || b == "."
	switch {
	case aEmpty && bEmpty:
		return path.Clean(c)
	case aEmpty:
		return path.Clean(b + "/" + c)
	case bEmpty:
		return path.Clean(a + "/" + c)
	default:
		return path.Clean(a + "/" + b + "/" + c)
	}
}

type match struct {
	prefix        string
	suffix        string
	originalPaths []string
}

type matchArray []match

func (s matchArray) Len() int {
	return len(s)
}
func (s matchArray) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// Sort the same as TypeScript/ESBuild prioritize longer prefixes and suffixes
// See https://github.com/evanw/esbuild/blob/deb93e92267a96575a6e434ff18421f4ef0605e4/internal/resolver/resolver.go#L1895-L1901
func (s matchArray) Less(i, j int) bool {
	return len(s[i].prefix) > len(s[j].prefix) || len(s[i].prefix) == len(s[j].prefix) && len(s[i].suffix) > len(s[j].suffix)
}
