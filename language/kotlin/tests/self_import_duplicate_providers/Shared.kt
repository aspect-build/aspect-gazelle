// This file makes the root target *also* a provider of `package test.shared`,
// alongside //earth and //moon. When the consumer below is resolved, the root
// target therefore appears among the import matches and must be filtered out as
// a self-import: the resulting conflict error should list only //earth and
// //moon, never the root target itself.
package test.shared

class Root
