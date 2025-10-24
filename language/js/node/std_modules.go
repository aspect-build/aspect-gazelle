package gazelle

import (
	_ "embed"
	"strings"

	"github.com/emirpasic/gods/sets/treeset"
)

//go:embed std_modules.list
var nativeModulesJson []byte

var nativeModulesSet = createNativeModulesSet()

func createNativeModulesSet() *treeset.Set {
	set := treeset.NewWithStringComparator()

	for m := range strings.SplitSeq(strings.TrimSpace(string(nativeModulesJson)), "\n") {
		set.Add(m)
	}

	return set
}

func IsNodeImport(imprt string) bool {
	return strings.HasPrefix(imprt, "node:") || nativeModulesSet.Contains(imprt)
}
