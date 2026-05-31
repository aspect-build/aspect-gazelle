import { greet } from "./greet";

// A trivial check so the test ts_project compiles and depends on ./greet.
const expected = "Hello, world!";
if (greet("world") !== expected) {
  throw new Error(`unexpected greeting: ${greet("world")}`);
}
