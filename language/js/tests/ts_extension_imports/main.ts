// Relative imports written with the real source extension (allowImportingTsExtensions).
import { lib } from './lib.ts';
import { view } from './view.tsx';
import { esm } from './esm.mts';
import { cjs } from './cjs.cts';
import { comp } from './comp.jsx';
export const main = [lib, view, esm, cjs, comp];
