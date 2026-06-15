package kotlinconfig

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bazel-contrib/rules_jvm/java/gazelle/javaconfig"
	"github.com/bazelbuild/bazel-gazelle/rule"
)

const Directive_KotlinExtension = "kotlin"

// GenericDirective is a version of [Directive] without type arguments.
type GenericDirective interface {
	// ConfigKey returns the string key used for this configuration directive
	// in gazelle comments.
	//
	// For a directive like "# gazelle:blah foo bar", returns "blah".
	ConfigKey() string

	// Parse parses the directive value and updates the config.
	Parse(dir rule.Directive, cfg *KotlinConfig) error
}

// Directive represents a Gazelle configuration directive for the Kotlin plugin.
// It maps a specific directive key (e.g. "kotlin_library_suffix") to a parser function
// and a setter function that updates the KotlinConfig accordingly.
type Directive[ParsedType any] struct {
	configKey string
	parseFn   func(d rule.Directive) (ParsedType, error)
	setFn     func(val ParsedType, cfg *KotlinConfig)
}

// ConfigKey returns the string key used for this configuration directive
// in gazelle comments.
//
// For a directive like "# gazelle:blah foo bar", returns "blah".
func (d *Directive[ParsedType]) ConfigKey() string { return d.configKey }

// parse parses the directive value.
func (d *Directive[ParsedType]) parse(dir rule.Directive) (ParsedType, error) {
	return d.parseFn(dir)
}

// Parse parses the directive value and updates the config.
func (d *Directive[ParsedType]) Parse(dir rule.Directive, cfg *KotlinConfig) error {
	val, err := d.parse(dir)
	if err != nil {
		return err
	}
	d.setFn(val, cfg)
	return nil
}

var librarySuffixRegexp = regexp.MustCompile(`^[^\s]*$`)

var (
	// The directive for enable or disabling the gazelle plugin.
	EnabledDirective = &Directive[bool]{
		Directive_KotlinExtension,
		parseEnabledDisableDirective,
		func(val bool, cfg *KotlinConfig) { cfg.SetGenerationEnabled(val) },
	}

	// A directive taking a single enabled/disabled argument that configures whether the
	// plugin should generate new library sources.
	OnlyUseExistingLibraryTargetsDirective = &Directive[bool]{
		"kotlin_only_use_existing_library_targets",
		parseEnabledDisableDirective,
		func(val bool, cfg *KotlinConfig) { cfg.SetOnlyUseExistingLibraryTargets(val) },
	}

	// A directive that configures the suffix used to name kt_jvm_library rules generated
	// by the plugin.
	LibraryRuleNameSuffix = &Directive[string]{
		"kotlin_library_suffix",
		func(d rule.Directive) (string, error) {
			value := strings.TrimSpace(d.Value)
			if !librarySuffixRegexp.MatchString(value) {
				return "", fmt.Errorf("invalid starlark name part %q - doesn't match regex %s", value, librarySuffixRegexp)
			}
			return value, nil
		},
		func(val string, cfg *KotlinConfig) { cfg.SetLibrarySuffix(val) },
	}

	// A directive that configures how the set of Kotlin identifiers associated
	// with a source file should be determined.
	//
	// Valid values:
	//
	// - "package": Default, specifies that the package statement of the source
	// file will be used to determine the set of Kotlin identifiers associated
	// with a source file (and that source files' Bazel target).
	//
	// - "top_level_objects": Specifies that the top-level objects defined in
	// the source files of the target will be used to determine the identifiers
	// associated with the target. For example, if a class Foo is defined
	// in a source file with package com.example, the "com.example.Foo"
	// identifier prefix would be associated with the library target of the
	// source file and used to resolve imports.
	ExportGranularityDirective = &Directive[ExportGranularity]{
		"kotlin_export_granularity",
		parseExportGranularity,
		func(val ExportGranularity, cfg *KotlinConfig) { cfg.SetExportGranularity(val) },
	}
)

// AllDirectives returns all directives defined by the kotlin plugin. This list excludes
// directives relevant to the Kotlin plugin but not defined by the plugin such as those
// defined by the rules_jvm plugin.
func AllDirectives() []GenericDirective {
	return []GenericDirective{
		EnabledDirective,
		OnlyUseExistingLibraryTargetsDirective,
		LibraryRuleNameSuffix,
		ExportGranularityDirective,
	}
}

func parseEnabledDisableDirective(d rule.Directive) (bool, error) {
	switch strings.TrimSpace(d.Value) {
	case "enabled":
		return true, nil
	case "disabled":
		return false, nil
	default:
		return false, fmt.Errorf("invalid directive value %q for key %q: expected enabled or disabled", d.Key, d.Value)
	}
}

// ExportGranularity defines valid values for the [ExportGranularityDirective].
type ExportGranularity string

const (
	// Package-level identifier resolution. See [ExportGranularityPackage]
	ExportGranularityPackage = "package"
	// Identifier resolution.
	ExportGranularityTopLevelObjects = "top_level_objects"
)

func parseExportGranularity(d rule.Directive) (ExportGranularity, error) {
	switch value := ExportGranularity(strings.TrimSpace(d.Value)); value {
	case ExportGranularityPackage, ExportGranularityTopLevelObjects:
		return value, nil
	default:
		return "", fmt.Errorf("invalid directive value %q for key %q: expected one of {%s, %s}", d.Key, d.Value, ExportGranularityPackage, ExportGranularityTopLevelObjects)
	}
}

type KotlinConfig struct {
	*javaconfig.Config

	parent *KotlinConfig
	rel    string

	librarySuffix    string
	testFileSuffixes []string

	generationEnabled             bool
	onlyUseExistingLibraryTargets bool

	exportGranularity ExportGranularity
}

type Configs = map[string]*KotlinConfig

func New(repoRoot string) *KotlinConfig {
	return &KotlinConfig{
		Config:            javaconfig.New(repoRoot),
		generationEnabled: true,
		parent:            nil,
		testFileSuffixes:  []string{"Test.kt"},
		librarySuffix:     "_lib",
		exportGranularity: ExportGranularityPackage,
	}
}

// String returns a debug string for the config.
func (c *KotlinConfig) String() string {
	return fmt.Sprintf("(KotlinConfig %q: enabled=%v; parent=\n  %s)", c.path(), c.generationEnabled, c.parent)
}

func (c *KotlinConfig) path() string {
	if c.parent == nil {
		return c.rel
	}
	return c.rel
}

// NewChild creates a new child Config. It inherits desired values from the
// current Config and sets itself as the parent to the child.
func (c *KotlinConfig) NewChild(childPath string) *KotlinConfig {
	cCopy := *c
	cCopy.Config = c.Config.NewChild()
	cCopy.rel = childPath
	cCopy.parent = c
	cCopy.testFileSuffixes = append([]string(nil), c.testFileSuffixes...)
	return &cCopy
}

// SetExportGranularity sets the export granularity for the config.
func (c *KotlinConfig) SetExportGranularity(granularity ExportGranularity) {
	c.exportGranularity = granularity
}

// ExportGranularity returns the export granularity for the config.
func (c *KotlinConfig) ExportGranularity() ExportGranularity {
	return c.exportGranularity
}

// SetGenerationEnabled sets whether the extension is enabled or not.
func (c *KotlinConfig) SetGenerationEnabled(enabled bool) {
	c.generationEnabled = enabled
}

// GenerationEnabled returns whether the extension is enabled or not.
func (c *KotlinConfig) GenerationEnabled() bool {
	return c.generationEnabled
}

// SetOnlyUseExistingLibraryTargets sets the value of the
// only-use-existing-library-targets configuration value.
func (c *KotlinConfig) SetOnlyUseExistingLibraryTargets(enabled bool) {
	c.onlyUseExistingLibraryTargets = enabled
}

// OnlyUseExistingLibraryTargets returns the value of the
// only-use-existing-library-targets configuration value.
func (c *KotlinConfig) OnlyUseExistingLibraryTargets() bool {
	return c.onlyUseExistingLibraryTargets
}

// SetLibrarySuffix sets the suffix to be appended to the names of kt_jvm_library
// targets generated by the plugin.
func (c *KotlinConfig) SetLibrarySuffix(suffix string) {
	c.librarySuffix = suffix
}

// LibrarySuffix returns the suffix of kt_jvm_library targets generated by the
// plugin.
func (c *KotlinConfig) LibrarySuffix() string {
	return c.librarySuffix
}

// IsTestBaseName reports if the given basename within the same bazel package
// as the config should be considered a test.
func (c *KotlinConfig) IsTestBaseName(baseName string) bool {
	for _, suffix := range c.testFileSuffixes {
		if strings.HasSuffix(baseName, suffix) {
			return true
		}
	}
	return false
}

// ParentForPackage returns the parent Config for the given Bazel package.
func ParentForPackage(c Configs, pkg string) *KotlinConfig {
	dir := filepath.Dir(pkg)
	if dir == "." {
		dir = ""
	}
	parent := (map[string]*KotlinConfig)(c)[dir]
	return parent
}
