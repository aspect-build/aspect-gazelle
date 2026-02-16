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

type StringOrStringArray []string

func (s *StringOrStringArray) UnmarshalJSON(data []byte) error {
	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		*s = []string{single}
		return nil
	}
	var multiple []string
	if err := json.Unmarshal(data, &multiple); err != nil {
		return fmt.Errorf("expected string or []string")
	}
	*s = multiple
	return nil
}

type tsConfigJSON struct {
	Extends         StringOrStringArray   `json:"extends"`
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
	Types []string

	// Extends lists all tsconfig files that this config directly extends.
	// Used for dependency tracking in Bazel. Ordered as they appear in the JSON.
	Extends []string

	// TODO: drop references? Not supported by rules_ts?
	References []string
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

// mergeBaseConfigs merges two parsed TsConfig objects for multiple extends support.
// The 'right' config overrides the 'left' config (TypeScript 5.0 semantics).
// Paths are adjusted to be relative to currentConfigDir.
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

	merged := &TsConfig{
		ConfigDir:  left.ConfigDir, // Use left's dir (arbitrary choice)
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
	if merged.DeclarationDir == "" || merged.DeclarationDir == "." {
		merged.DeclarationDir = left.DeclarationDir
	}

	merged.OutDir = right.OutDir
	if merged.OutDir == "" || merged.OutDir == "." {
		merged.OutDir = left.OutDir
	}

	merged.RootDir = right.RootDir
	if merged.RootDir == "" || merged.RootDir == "." {
		merged.RootDir = left.RootDir
	}

	merged.BaseUrl = right.BaseUrl
	if merged.BaseUrl == "" || merged.BaseUrl == "." {
		merged.BaseUrl = left.BaseUrl
	}

	merged.TsBuildInfoFile = right.TsBuildInfoFile
	if merged.TsBuildInfoFile == "" {
		merged.TsBuildInfoFile = left.TsBuildInfoFile
	}

	// Paths: right completely replaces left (TypeScript behavior)
	if right.Paths != nil && right.Paths != &DefaultConfigPaths && len(right.Paths.Map) > 0 {
		merged.Paths = &TsConfigPaths{
			Rel: path.Join(rightRel, right.Paths.Rel),
			Map: right.Paths.Map,
		}
	} else if left.Paths != nil && left.Paths != &DefaultConfigPaths && len(left.Paths.Map) > 0 {
		merged.Paths = &TsConfigPaths{
			Rel: path.Join(leftRel, left.Paths.Rel),
			Map: left.Paths.Map,
		}
	} else {
		merged.Paths = &DefaultConfigPaths
	}

	// VirtualRootDirs: right completely replaces left
	if len(right.VirtualRootDirs) > 0 {
		merged.VirtualRootDirs = make([]string, len(right.VirtualRootDirs))
		for i, d := range right.VirtualRootDirs {
			merged.VirtualRootDirs[i] = path.Join(rightRel, d)
		}
	} else if len(left.VirtualRootDirs) > 0 {
		merged.VirtualRootDirs = make([]string, len(left.VirtualRootDirs))
		for i, d := range left.VirtualRootDirs {
			merged.VirtualRootDirs[i] = path.Join(leftRel, d)
		}
	}

	// ImportHelpers: right overrides left
	// Since ImportHelpers is a bool (not *bool), we can't distinguish between
	// "not set" and "set to false". So we trust that the parsed configs have
	// the correct values and just take the right value.
	merged.ImportHelpers = right.ImportHelpers

	// Jsx: right overrides left if set
	merged.Jsx = right.Jsx
	if merged.Jsx == JsxNone {
		merged.Jsx = left.Jsx
	}

	// Types: right completely replaces left
	if len(right.Types) > 0 {
		merged.Types = right.Types
	} else {
		merged.Types = left.Types
	}

	// References: right completely replaces left
	if len(right.References) > 0 {
		merged.References = right.References
	} else {
		merged.References = left.References
	}

	// Extends: concatenate both extends lists (all dependencies)
	merged.Extends = append(left.Extends, right.Extends...)

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

	// Parse and merge all extended configs (if any)
	var baseConfig *TsConfig
	var extendsClean []string

	configDir := path.Dir(tsconfig)
	configName := path.Base(tsconfig)

	if len(c.Extends) > 0 {
		extendsClean = make([]string, 0, len(c.Extends))

		// Copy and clean all extends paths for dependency tracking
		// Filter out empty strings
		for _, ext := range c.Extends {
			if ext != "" {
				extendsClean = append(extendsClean, path.Clean(ext))
			}
		}

		// Load all base configs
		baseConfigs := make([]*TsConfig, 0, len(c.Extends))

		for _, ext := range c.Extends {
			var loadedBase *TsConfig

			// Try to resolve and load this extended config
			for _, potential := range resolver(path.Dir(tsconfig), ext) {
				base, err := parseTsConfigJSONFile(parsed, resolver, root, potential)

				if err != nil {
					BazelLog.Warnf("Failed to load base tsconfig file %q from %q: %v", ext, tsconfig, err)
				} else if base != nil {
					loadedBase = base
					break
				}
			}

			if loadedBase != nil {
				baseConfigs = append(baseConfigs, loadedBase)
			}
		}

		// Merge all base configs left-to-right (later overrides earlier)
		if len(baseConfigs) > 0 {
			baseConfig = baseConfigs[0]
			for i := 1; i < len(baseConfigs); i++ {
				baseConfig = mergeBaseConfigs(baseConfig, baseConfigs[i], configDir)
			}
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
	if c.CompilerOptions.TsBuildInfoFile != nil {
		tsBuildInfoFile = expandConfigDirFile(*c.CompilerOptions.TsBuildInfoFile)
	} else if baseConfig != nil {
		tsBuildInfoFile = baseConfig.TsBuildInfoFile
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
	if c.CompilerOptions.RootDir != nil {
		RootDir = expandConfigDirPath(*c.CompilerOptions.RootDir)
	} else if baseConfig != nil {
		RootDir = baseConfig.RootDir
	} else {
		RootDir = "."
	}

	var OutDir string
	if c.CompilerOptions.OutDir != nil {
		OutDir = expandConfigDirPath(*c.CompilerOptions.OutDir)
	} else if baseConfig != nil {
		OutDir = baseConfig.OutDir
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
	if c.CompilerOptions.BaseUrl != nil {
		BaseUrl = expandConfigDirPath(*c.CompilerOptions.BaseUrl)
	} else {
		BaseUrl = "."
	}

	var Paths *TsConfigPaths
	if c.CompilerOptions.Paths != nil {
		Paths = &TsConfigPaths{
			Rel: BaseUrl,
			Map: *c.CompilerOptions.Paths,
		}
	} else if baseConfig != nil {
		Paths = &TsConfigPaths{
			Rel: path.Join(baseConfigRel, baseConfig.Paths.Rel),
			Map: baseConfig.Paths.Map,
		}
	} else {
		Paths = &DefaultConfigPaths
	}

	var VirtualRootDirs []string
	if c.CompilerOptions.RootDirs != nil {
		for _, d := range *c.CompilerOptions.RootDirs {
			VirtualRootDirs = append(VirtualRootDirs, path.Clean(d))
		}
	} else if baseConfig != nil {
		for _, d := range baseConfig.VirtualRootDirs {
			VirtualRootDirs = append(VirtualRootDirs, path.Join(baseConfigRel, d))
		}
	}

	var importHelpers = false
	if c.CompilerOptions.ImportHelpers != nil {
		importHelpers = *c.CompilerOptions.ImportHelpers
	} else if baseConfig != nil {
		importHelpers = baseConfig.ImportHelpers
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
		Extends:              extendsClean,
		ImportHelpers:        importHelpers,
		Jsx:                  jsx,
		Types:                types,
		References:           references,
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
