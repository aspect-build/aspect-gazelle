package plugin

import "sync"

// TODO: move to its own package

type Database struct {
	symbols map[string][]TargetSymbol

	mu sync.RWMutex
}

func (d *Database) AddSymbol(label Label, symbol Symbol) {
	d.add(TargetSymbol{Symbol: symbol, Label: label})
}

// AddSymbolImport registers a symbol provided by whatever target(s) `imp`
// resolves to; the import is resolved lazily at resolve time. `pkg` is the
// declaring package (Repo/Pkg only), used to rank same-id symbols by nearness.
func (d *Database) AddSymbolImport(pkg Label, symbol Symbol, imp TargetImport) {
	d.add(TargetSymbol{Symbol: symbol, Label: pkg, Import: &imp})
}

func (d *Database) add(s TargetSymbol) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.symbols == nil {
		d.symbols = make(map[string][]TargetSymbol)
	}
	d.symbols[s.Id] = append(d.symbols[s.Id], s)
}

// LookupSymbols returns all symbols registered with the given id.
func (d *Database) LookupSymbols(id string) []TargetSymbol {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.symbols[id]
}
