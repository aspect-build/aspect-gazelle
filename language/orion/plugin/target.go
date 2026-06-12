package plugin

type Symbol struct {
	Id       string // The unique id of the symbol
	Provider string // The provider type of the symbol
}

type TargetImport struct {
	Symbol

	// Multiple resolves the Symbol to every label that provides it instead of requiring a single match.
	Multiple bool

	// Optional imports will not be treated as resolution errors when not found.
	Optional bool

	// Ancestor resolves to the closest rule providing this Symbol in the importing rule's package or an ancestor.
	// With ancestor=True the resolver tries Import.Id prefixed with the importing package and each of its ancestors,
	// ending with the bare Import.Id at the workspace root (eg for Import.Id="tsconfig.json" from //a/b: tries
	// "a/b/tsconfig.json", "a/tsconfig.json", "tsconfig.json"). Producers should declare Symbol(id) with the
	// matching full path (eg path.join(ctx.rel, "tsconfig.json")).
	Ancestor bool

	// Where the import is from such as file path, for debugging
	From string
}

type TargetSymbol struct {
	Symbol

	// The label producing the symbol
	Label Label
}

/**
 * A bazel target declaration describing the target name/type/attributes as
 * well as symbols representing imports and exports of the target.
 */
type TargetDeclaration struct {
	Name  string
	Kind  string
	Attrs map[string]any

	// Names (possibly as paths) exported from this target
	Symbols []Symbol
}

type TargetAction any

type AddTargetAction struct {
	TargetDeclaration
}

type RemoveTargetAction struct {
	Name string
	Kind string
}
