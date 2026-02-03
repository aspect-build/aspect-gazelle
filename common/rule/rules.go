package rule

import (
	"fmt"

	BazelLog "github.com/aspect-build/aspect-gazelle/common/logger"
	"github.com/bazelbuild/bazel-gazelle/label"
	"github.com/bazelbuild/bazel-gazelle/language"
	"github.com/bazelbuild/bazel-gazelle/rule"
	"github.com/emirpasic/gods/v2/sets/treeset"
)

func GetFileRuleByName(args language.GenerateArgs, ruleName string) *rule.Rule {
	if args.File == nil {
		return nil
	}

	for _, r := range args.File.Rules {
		if r.Name() == ruleName {
			return r
		}
	}

	return nil
}

func MapKind(args language.GenerateArgs, kind string) string {
	mappedKind := args.Config.KindMap[kind].KindName
	if mappedKind != "" {
		return mappedKind
	}

	return kind
}

func RemoveRule(args language.GenerateArgs, ruleName string, generatedKinds *treeset.Set[string], result *language.GenerateResult) {
	existing := GetFileRuleByName(args, ruleName)
	if existing == nil {
		BazelLog.Tracef("remove rule '%s:%s' not found", args.Rel, ruleName)
		return
	}

	// Only remove rules controlled by this gazelle plugin
	if mappedKind, isMapped := getMappedKind(args, generatedKinds, existing.Kind()); isMapped {
		BazelLog.Infof("remove rule '%s:%s' (%q mapped as %q)", args.Rel, existing.Name(), mappedKind, existing.Kind())

		emptyRule := rule.NewRule(mappedKind, ruleName)
		result.Empty = append(result.Empty, emptyRule)
	}
}

// Check if a target with the same name we are generating already exists,
// and that rule type is unknown or can not be adapted to the new rule kind.
// If an existing rule can not be adapted (maybe due to Gazelle bugs/limitations) an
// error explaining the case is returned.
func CheckCollisionErrors(targetName, expectedKind string, generatedKinds *treeset.Set[string], args language.GenerateArgs) error {
	// No file generated yet
	if args.File == nil {
		return nil
	}

	existing := GetFileRuleByName(args, targetName)

	// No rule of the same name
	if existing == nil {
		return nil
	}

	if !containsMappedKind(args, generatedKinds, existing.Kind()) {
		mappedExpectedKind := MapKind(args, expectedKind)

		fqTarget := label.New("", args.Rel, targetName)
		return fmt.Errorf("failed to generate target %q of kind %q: "+
			"a target of kind %q with the same name already exists.",
			fqTarget.String(), mappedExpectedKind, existing.Kind())
	}

	return nil
}

func containsMappedKind(args language.GenerateArgs, generatedKinds *treeset.Set[string], kind string) bool {
	_, found := getMappedKind(args, generatedKinds, kind)
	return found
}

func getMappedKind(args language.GenerateArgs, generatedKinds *treeset.Set[string], kind string) (string, bool) {
	for it := generatedKinds.Iterator(); it.Next(); {
		generatedKind := it.Value()
		if MapKind(args, generatedKind) == kind {
			return generatedKind, true
		}
	}

	return "", false
}
