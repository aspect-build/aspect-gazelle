// The import should still be extracted (and resolved to a dep) even
// though there is a syntax error elsewhere in the file.
import { dep } from './dep';

export function main() {
    looks like some bad syntax here
}

console.log(dep);
