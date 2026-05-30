// Regular top-level require() invocation.
const b = require('@aspect-test/b');

// TypeScript `import x = require(...)` is a different AST than a regular require()
import c = require('@aspect-test/c');

console.log(b, c);
