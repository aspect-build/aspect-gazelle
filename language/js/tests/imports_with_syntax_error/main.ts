// A file with a valid import but a syntax error.
import { dep } from './dep';

export function main() {
    looks like some bad syntax here
}

console.log(dep);
