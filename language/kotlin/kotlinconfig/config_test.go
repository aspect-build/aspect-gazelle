package kotlinconfig_test

import (
	"testing"

	"github.com/aspect-build/aspect-gazelle/language/kotlin/kotlinconfig"
	"github.com/bazelbuild/bazel-gazelle/rule"
)

func TestDirectives(t *testing.T) {
	testCases := []struct {
		name          string
		directive     rule.Directive
		expectedError string
		check         func(t *testing.T, cfg *kotlinconfig.KotlinConfig)
	}{
		{
			name:      "enabled",
			directive: rule.Directive{Key: "kotlin", Value: "enabled"},
			check: func(t *testing.T, cfg *kotlinconfig.KotlinConfig) {
				if !cfg.GenerationEnabled() {
					t.Errorf("expected GenerationEnabled to be true")
				}
			},
		},
		{
			name:      "disabled",
			directive: rule.Directive{Key: "kotlin", Value: "disabled"},
			check: func(t *testing.T, cfg *kotlinconfig.KotlinConfig) {
				if cfg.GenerationEnabled() {
					t.Errorf("expected GenerationEnabled to be false")
				}
			},
		},
		{
			name:      "generate_mode package",
			directive: rule.Directive{Key: "kotlin_generate_mode", Value: "package"},
			check: func(t *testing.T, cfg *kotlinconfig.KotlinConfig) {
				if cfg.GenerateMode() != kotlinconfig.GenerateModePackage {
					t.Errorf("expected GenerateMode to be %q, got %q", kotlinconfig.GenerateModePackage, cfg.GenerateMode())
				}
			},
		},
		{
			name:      "generate_mode file",
			directive: rule.Directive{Key: "kotlin_generate_mode", Value: "file"},
			check: func(t *testing.T, cfg *kotlinconfig.KotlinConfig) {
				if cfg.GenerateMode() != kotlinconfig.GenerateModeFile {
					t.Errorf("expected GenerateMode to be %q, got %q", kotlinconfig.GenerateModeFile, cfg.GenerateMode())
				}
			},
		},
		{
			name:      "generate_mode existing",
			directive: rule.Directive{Key: "kotlin_generate_mode", Value: "existing"},
			check: func(t *testing.T, cfg *kotlinconfig.KotlinConfig) {
				if cfg.GenerateMode() != kotlinconfig.GenerateModeExisting {
					t.Errorf("expected GenerateMode to be %q, got %q", kotlinconfig.GenerateModeExisting, cfg.GenerateMode())
				}
			},
		},
		{
			name:      "library_suffix",
			directive: rule.Directive{Key: "kotlin_library_suffix", Value: "_custom_suffix"},
			check: func(t *testing.T, cfg *kotlinconfig.KotlinConfig) {
				if cfg.LibrarySuffix() != "_custom_suffix" {
					t.Errorf("expected LibrarySuffix to be '_custom_suffix', got %q", cfg.LibrarySuffix())
				}
			},
		},
		{
			name:          "library_suffix invalid",
			directive:     rule.Directive{Key: "kotlin_library_suffix", Value: "bad suffix"},
			expectedError: "invalid starlark name part",
		},
		{
			name:      "resolve_granularity package",
			directive: rule.Directive{Key: "kotlin_resolve_granularity", Value: "package"},
			check: func(t *testing.T, cfg *kotlinconfig.KotlinConfig) {
				if cfg.ResolveGranularity() != kotlinconfig.ResolveGranularityPackage {
					t.Errorf("expected ResolveGranularity to be %q, got %q", kotlinconfig.ResolveGranularityPackage, cfg.ResolveGranularity())
				}
			},
		},
		{
			name:      "resolve_granularity symbol",
			directive: rule.Directive{Key: "kotlin_resolve_granularity", Value: "symbol"},
			check: func(t *testing.T, cfg *kotlinconfig.KotlinConfig) {
				if cfg.ResolveGranularity() != kotlinconfig.ResolveGranularitySymbol {
					t.Errorf("expected ResolveGranularity to be %q, got %q", kotlinconfig.ResolveGranularitySymbol, cfg.ResolveGranularity())
				}
			},
		},
		{
			name:          "resolve_granularity invalid",
			directive:     rule.Directive{Key: "kotlin_resolve_granularity", Value: "unknown"},
			expectedError: "invalid directive value",
		},
	}

	directivesByKey := make(map[string]kotlinconfig.GenericDirective)
	for _, dir := range kotlinconfig.AllDirectives() {
		directivesByKey[dir.ConfigKey()] = dir
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := kotlinconfig.New("workspace")
			
			dir, ok := directivesByKey[tc.directive.Key]
			if !ok {
				t.Fatalf("unknown directive key %q", tc.directive.Key)
			}

			err := dir.Parse(tc.directive, cfg)
			
			if tc.expectedError != "" {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tc.expectedError)
				} else if err.Error() == "" || !contains(err.Error(), tc.expectedError) {
					t.Errorf("expected error containing %q, got %v", tc.expectedError, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tc.check != nil {
				tc.check(t, cfg)
			}
		})
	}
}

func contains(s, substr string) bool {
	// simple helper since strings.Contains is in "strings" package
	// just to avoid another import or we can import strings.
	return len(s) >= len(substr) && func() bool {
		for i := 0; i <= len(s)-len(substr); i++ {
			if s[i:i+len(substr)] == substr {
				return true
			}
		}
		return false
	}()
}
