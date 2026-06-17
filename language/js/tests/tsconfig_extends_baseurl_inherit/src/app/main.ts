// This config extends ../../tsconfig.json and does NOT set its own baseUrl, so
// per tsc it inherits baseUrl="./src" (anchored at the base config's dir, i.e.
// the workspace root). A bare import of 'lib/util' must therefore resolve to
// src/lib/util -> //src/lib/util, NOT to the same-named decoy under this dir.
import { util } from 'lib/util';

console.log(util);
