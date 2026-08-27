// Various import forms nested below the program root, so each form must still
// be discovered as a dependency.
namespace N {
	const a = require('@aspect-test/a'); // CommonJS require() call
	export const j = import('jquery'); // dynamic import()

	export const all = [a, j];
}

// import-equals is only valid at the top level or in an ambient module
// declaration (TS1147 inside a namespace).
declare module 'fake-ambient' {
	import c = require('@aspect-test/c'); // import = require (import-equals)
	export const x: typeof c;
}

console.log(N.all);
