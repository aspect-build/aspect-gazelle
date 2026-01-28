// Import current directory's index.ts using '.'
exports.nested_index = require(".").nested_index;

// Import parent directory's index.ts using '..'
exports.subproject_index = require("..").subproject_index;
