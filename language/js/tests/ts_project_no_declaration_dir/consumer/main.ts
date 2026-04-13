// Import via the root-dir-stripped path, NOT the out_dir path (../lib/dist/util).
// This path is only indexed when declaration_dir incorrectly defaults to "."
// instead of out_dir, so this dep should be unresolved and produce no output.
import { util } from '../lib/util';
export const consumer = util;
