// Absolute imports starting with / are treated as workspace-root-relative

// Import from workspace root
const { subproject_index } = await import("/subproject");
const { subproject_lib } = await import("/subdir/lib");

// With .js extension
const { subproject_lib: lib2 } = await import("/subproject/lib.js");

// With query parameters (should be stripped)
const { subproject_lib: lib3 } = await import("/subproject-index");
const { subproject_lib: lib4 } = await import("/subproject/lib#bar");

export const absolute = {
    subproject_index,
    subproject_lib,
    lib2,
    lib3,
    lib4,
};
