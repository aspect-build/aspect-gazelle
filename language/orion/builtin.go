package gazelle

import (
	plugin "github.com/aspect-build/aspect-gazelle/language/orion/plugin"
)

var builtinKinds = []plugin.RuleKind{
	// Native
	// TODO: remove once https://github.com/bazel-contrib/bazel-gazelle/pull/2053 lands
	plugin.RuleKind{
		Name: "filegroup",
		KindInfo: plugin.KindInfo{
			NonEmptyAttrs:  []string{"srcs"},
			MergeableAttrs: []string{"srcs"},
		},
	},

	// @bazel_lib
	plugin.RuleKind{
		Name: "copy_to_bin",
		From: "@bazel_lib//lib:copy_to_bin.bzl",
		KindInfo: plugin.KindInfo{
			NonEmptyAttrs:  []string{"srcs"},
			MergeableAttrs: []string{"srcs"},
		},
	},
}
