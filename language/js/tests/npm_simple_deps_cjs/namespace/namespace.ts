// Various import forms nested inside a TypeScript `namespace {}` block. These
// live below the program root, so each valid in-namespace form must still be
// discovered as a dependency.
namespace N {
	import c = require('@aspect-test/c'); // import = require (import-equals)
	const a = require('@aspect-test/a'); // CommonJS require() call
	export const j = import('jquery'); // dynamic import()

	export const all = [c, a, j];
}

console.log(N.all);
