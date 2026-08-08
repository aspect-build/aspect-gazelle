// The .jsx index file imported as a directory
import { Component } from './lib';

// The .jsx source imported as its transpiled .js output
import { Component as Direct } from './lib/component.js';

// The .jsx source imported as its generated .d.ts output
import type { Component as Typed } from './lib/component.d.ts';

export { Component, Direct };
export type { Typed };
