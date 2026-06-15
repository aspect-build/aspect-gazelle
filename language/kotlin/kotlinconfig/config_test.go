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
			name:      "only_use_existing_library_targets enabled",
			directive: rule.Directive{Key: "kotlin_only_use_existing_library_targets", Value: "enabled"},
			check: func(t *testing.T, cfg *kotlinconfig.KotlinConfig) {
				if !cfg.OnlyUseExistingLibraryTargets() {
					t.Errorf("expected OnlyUseExistingLibraryTargets to be true")
				}
			},
		},
		{
			name:      "only_use_existing_library_targets disabled",
			directive: rule.Directive{Key: "kotlin_only_use_existing_library_targets", Value: "disabled"},
			check: func(t *testing.T, cfg *kotlinconfig.KotlinConfig) {
				if cfg.OnlyUseExistingLibraryTargets() {
					t.Errorf("expected OnlyUseExistingLibraryTargets to be false")
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
			name:      "export_granularity package",
			directive: rule.Directive{Key: "kotlin_export_granularity", Value: "package"},
			check: func(t *testing.T, cfg *kotlinconfig.KotlinConfig) {
				if cfg.ExportGranularity() != kotlinconfig.ExportGranularityPackage {
					t.Errorf("expected ExportGranularity to be %q, got %q", kotlinconfig.ExportGranularityPackage, cfg.ExportGranularity())
				}
			},
		},
		{
			name:      "export_granularity top_level_objects",
			directive: rule.Directive{Key: "kotlin_export_granularity", Value: "top_level_objects"},
			check: func(t *testing.T, cfg *kotlinconfig.KotlinConfig) {
				if cfg.ExportGranularity() != kotlinconfig.ExportGranularityTopLevelObjects {
					t.Errorf("expected ExportGranularity to be %q, got %q", kotlinconfig.ExportGranularityTopLevelObjects, cfg.ExportGranularity())
				}
			},
		},
		{
			name:          "export_granularity invalid",
			directive:     rule.Directive{Key: "kotlin_export_granularity", Value: "unknown"},
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
