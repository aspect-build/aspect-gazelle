// tsc resolves these css-module imports to the generated typings under
// gentypes/ via the tsconfig rootDirs mapping ("./x.module.css" probes
// "<rootDir>/x.module.css.d.ts" in each rootDir).
import ownStyles from './styles.module.css';
import subStyles from './sub/styles.module.css';

// A real source at the literal location wins over virtual typings: tsc probes
// the importing file's own rootDir first, so this resolves to ./sub/util.ts
// (a //sub dep) even though gentypes/sub/util.d.ts also exists.
import { util } from './sub/util';

console.log(ownStyles.own, subStyles.sub, util);
