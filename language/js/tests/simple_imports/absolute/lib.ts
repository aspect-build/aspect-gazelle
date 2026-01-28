// Absolute imports starting with / are treated as workspace-root-relative

// Import from workspace root
import { subproject_index } from "/subproject";
import { subproject_lib } from "/subdir/lib";

// With .js extension
import { subproject_lib as lib2 } from "/subproject/lib.js";

// With query parameters (should be stripped)
import { subproject_lib as lib3 } from "/subproject-index";
import { subproject_lib as lib4 } from "/subproject/lib#bar";

export const absolute = {
    subproject_index,
    subproject_lib,
    lib2,
    lib3,
    lib4,
};
