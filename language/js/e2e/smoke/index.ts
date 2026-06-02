import { greet } from "./greet";

// Import an npm package (tslib) so the gazelle js extension resolves it to
// //:node_modules/tslib in the generated ts_project deps.
import { __spreadArray } from "tslib";

export function greetAll(names: string[]): string[] {
  return __spreadArray([], names, true).map((name) => greet(name));
}
