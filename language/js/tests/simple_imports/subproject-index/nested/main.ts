// Import current directory's index.ts using '.'
import { nested_index } from ".";

// Import parent directory's index.ts using '..'
import { subproject_index } from "..";

export const nested_lib = {
    nested_index,
    subproject_index,
};
