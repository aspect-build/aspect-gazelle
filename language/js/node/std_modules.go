package node

import (
	_ "embed"
	"strings"
)

//go:embed std_modules.list
var nativeModulesJson []byte

var nativeModulesSet = createNativeModulesSet()

func createNativeModulesSet() map[string]struct{} {
	set := make(map[string]struct{})

	for m := range strings.SplitSeq(strings.TrimSpace(string(nativeModulesJson)), "\n") {
		set[m] = struct{}{}
	}

	return set
}

func IsNodeImport(imprt string) bool {
	if strings.HasPrefix(imprt, "node:") {
		return true
	}
	_, ok := nativeModulesSet[imprt]
	return ok
}
