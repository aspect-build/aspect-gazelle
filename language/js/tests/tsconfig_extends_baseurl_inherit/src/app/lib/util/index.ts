// Decoy: a same-named module sitting under src/app/. It is only reachable if
// baseUrl is (incorrectly) reset to the extending config's own directory.
export const util = 'the WRONG util, under src/app/';
