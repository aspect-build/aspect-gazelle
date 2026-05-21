package plugin

import "sync"

// TODO: move to its own package

type Database struct {
	symbols map[string][]TargetSymbol

	mu sync.RWMutex
}

func (d *Database) AddSymbol(label Label, symbol Symbol) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.symbols == nil {
		d.symbols = make(map[string][]TargetSymbol)
	}
	d.symbols[symbol.Id] = append(d.symbols[symbol.Id], TargetSymbol{
		Symbol: symbol,
		Label:  label,
	})
}

// LookupSymbols returns all symbols registered with the given id.
func (d *Database) LookupSymbols(id string) []TargetSymbol {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.symbols[id]
}
