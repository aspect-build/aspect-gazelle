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

	// GenerateModeDirective configures the target generation style.
	//
	// Valid values:
	//
	// - "package": Default. Gazelle runs in automatic target-generation mode, constructing
	// and inserting a kt_jvm_library target for directories containing Kotlin source files.
	//
	// - "file": Auto-generate one target per Kotlin source file (not yet implemented).
	//
	// - "existing": Gazelle operates in strict mode. It updates existing library targets but
	// refuses to create new ones. If a Kotlin file is not mapped to any existing library
	// target's srcs, Gazelle fails with an error. To skip files, exclude them using
	// '# gazelle:exclude' or .gitignore.
	GenerateModeDirective = &Directive[GenerateMode]{
		"kotlin_generate_mode",
		parseGenerateMode,
		func(val GenerateMode, cfg *KotlinConfig) { cfg.SetGenerateMode(val) },
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

	// ResolveGranularityDirective configures how the set of Kotlin identifiers associated
	// with a source file should be determined.
	//
	// Valid values:
	//
	// - "package": Default (to be updated to symbol). Specifies that the package statement
	// of the source file will be used to determine the set of Kotlin identifiers associated
	// with a source file (and that source files' Bazel target).
	//
	// - "symbol": Specifies that the top-level declarations (classes, interfaces,
	// singleton objects, functions, properties, and typealiases) defined in the source
	// files of the target will be used to determine the identifiers associated with the target.
	ResolveGranularityDirective = &Directive[ResolveGranularity]{
		"kotlin_resolve_granularity",
		parseResolveGranularity,
		func(val ResolveGranularity, cfg *KotlinConfig) { cfg.SetResolveGranularity(val) },
	}
)

// AllDirectives returns all directives defined by the kotlin plugin. This list excludes
// directives relevant to the Kotlin plugin but not defined by the plugin such as those
// defined by the rules_jvm plugin.
func AllDirectives() []GenericDirective {
	return []GenericDirective{
		EnabledDirective,
		GenerateModeDirective,
		LibraryRuleNameSuffix,
		ResolveGranularityDirective,
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

// GenerateMode defines valid values for the [GenerateModeDirective].
type GenerateMode string

const (
	GenerateModePackage  GenerateMode = "package"
	GenerateModeFile     GenerateMode = "file"
	GenerateModeExisting GenerateMode = "existing"
)

func parseGenerateMode(d rule.Directive) (GenerateMode, error) {
	switch value := GenerateMode(strings.TrimSpace(d.Value)); value {
	case GenerateModePackage, GenerateModeFile, GenerateModeExisting:
		return value, nil
	default:
		return "", fmt.Errorf("invalid directive value %q for key %q: expected one of {%s, %s, %s}", d.Value, d.Key, GenerateModePackage, GenerateModeFile, GenerateModeExisting)
	}
}

// ResolveGranularity defines valid values for the [ResolveGranularityDirective].
type ResolveGranularity string

const (
	ResolveGranularityPackage ResolveGranularity = "package"
	ResolveGranularitySymbol  ResolveGranularity = "symbol"
)

func parseResolveGranularity(d rule.Directive) (ResolveGranularity, error) {
	switch value := ResolveGranularity(strings.TrimSpace(d.Value)); value {
	case ResolveGranularityPackage, ResolveGranularitySymbol:
		return value, nil
	default:
		return "", fmt.Errorf("invalid directive value %q for key %q: expected one of {%s, %s}", d.Value, d.Key, ResolveGranularityPackage, ResolveGranularitySymbol)
	}
}

// KotlinConfig holds the configuration settings for the Kotlin Gazelle plugin
// within a specific Bazel package. It embeds the Java configuration and maintains
// hierarchical inheritance via its parent field.
type KotlinConfig struct {
	*javaconfig.Config

	// parent is the configuration of the parent package directory,
	// used to inherit configuration settings down the directory hierarchy.
	parent *KotlinConfig

	// rel is the package path relative to the repository root.
	rel string

	// librarySuffix is the suffix appended to the folder name to produce
	// target names for auto-generated `kt_jvm_library` rules (e.g. "_lib").
	librarySuffix string

	// testFileSuffixes defines the set of file name suffixes (e.g. "Test.kt")
	// that classify a source file as a test file.
	testFileSuffixes []string

	// generationEnabled indicates whether Gazelle target generation is active
	// for the current package.
	generationEnabled bool

	// generateMode defines the style of target generation.
	generateMode GenerateMode

	// resolveGranularity defines whether targets publish/resolve imports at
	// the package prefix level or at the individual top-level symbol level.
	resolveGranularity ResolveGranularity
}

type Configs = map[string]*KotlinConfig

func New(repoRoot string) *KotlinConfig {
	return &KotlinConfig{
		Config:             javaconfig.New(repoRoot),
		generationEnabled:  true,
		parent:             nil,
		testFileSuffixes:   []string{"Test.kt"},
		librarySuffix:      "_lib",
		generateMode:       GenerateModePackage,
		resolveGranularity: ResolveGranularityPackage,
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

// SetResolveGranularity sets the resolve granularity for the config.
func (c *KotlinConfig) SetResolveGranularity(granularity ResolveGranularity) {
	c.resolveGranularity = granularity
}

// ResolveGranularity returns the resolve granularity for the config.
func (c *KotlinConfig) ResolveGranularity() ResolveGranularity {
	return c.resolveGranularity
}

// SetGenerationEnabled sets whether the extension is enabled or not.
func (c *KotlinConfig) SetGenerationEnabled(enabled bool) {
	c.generationEnabled = enabled
}

// GenerationEnabled returns whether the extension is enabled or not.
func (c *KotlinConfig) GenerationEnabled() bool {
	return c.generationEnabled
}

// SetGenerateMode sets the generate mode for the config.
func (c *KotlinConfig) SetGenerateMode(mode GenerateMode) {
	c.generateMode = mode
}

// GenerateMode returns the generate mode for the config.
func (c *KotlinConfig) GenerateMode() GenerateMode {
	return c.generateMode
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
