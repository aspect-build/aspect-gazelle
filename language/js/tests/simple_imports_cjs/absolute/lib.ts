// Absolute imports starting with / are treated: workspace-root-relative

// Import = require(workspace root
const { subproject_index } = require("/subproject");
const { subproject_lib } = require("/subdir/lib");

// With .js extension
const { subproject_lib: lib2 } = require("/subproject/lib.js");

// With query parameters (should be stripped)
const { subproject_lib: lib3 } = require("/subproject-index");
const { subproject_lib: lib4 } = require("/subproject/lib#bar");

exports.absolute = {
    subproject_index,
    subproject_lib,
    lib2,
    lib3,
    lib4,
});
