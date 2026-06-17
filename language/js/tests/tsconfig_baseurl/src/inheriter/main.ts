// The local tsconfig.json extends ../../tsconfig.json without setting its own
// baseUrl, so it inherits baseUrl="./src" (anchored at the base config's dir).
// Bare specifiers resolve against that inherited baseUrl: 'inheriter/lib/d' ->
// src/inheriter/lib/d.
import { D } from 'inheriter/lib/d';
import { C1 } from '../lib/c/first/c1';

console.log(D, C1);
