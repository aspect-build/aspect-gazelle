// This module has no Go sources of its own; it exists only so this root MODULE
// can resolve `common` (consumed by the runner's gazelle binary via go_deps as
// @com_github_aspect_build_aspect_gazelle_common) from local source. A go_deps
// `replace` is only honored from the root module, so without this the build
// would fetch a stale published `common` from the network.
module github.com/aspect-build/aspect-gazelle/runner/e2e/bin

go 1.26.3

require github.com/aspect-build/aspect-gazelle/common v0.0.0-20260315054354-c9bd89ae01a1

replace github.com/aspect-build/aspect-gazelle/common => ../../../common
