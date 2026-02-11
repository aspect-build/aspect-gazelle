package plugin

type Symbol struct {
	Id       string // The unique id of the symbol
	Provider string // The provider type of the symbol
}

type TargetImport struct {
	Symbol

	// Optional imports will not be treated as resolution errors when not found.
	Optional bool

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
	TargetAction
	TargetDeclaration
}

type RemoveTargetAction struct {
	TargetAction
	Name string
	Kind string
}
