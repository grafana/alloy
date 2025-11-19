// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package libpf // import "go.opentelemetry.io/ebpf-profiler/libpf"

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"go.opentelemetry.io/ebpf-profiler/util"
)

// List of public errors.
var (
	// ErrSymbolNotFound is returned when the requested symbol was not found.
	ErrSymbolNotFound = errors.New("symbol not found")
)

// SymbolValue represents the value associated with a symbol, e.g. either an
// offset or an absolute address
type SymbolValue uint64

// SymbolName represents the name of a symbol
type SymbolName string

// SymbolValueInvalid is the value returned by SymbolMap functions when symbol was not found.
const SymbolValueInvalid = SymbolValue(0)

// SymbolNameUnknown is the value returned by SymbolMap functions when address has no symbol info.
const SymbolNameUnknown = ""

// SymbolFinder implements a way to find symbol data
type SymbolFinder interface {
	LookupSymbol(symbolName SymbolName) (*Symbol, error)

	LookupSymbolAddress(symbolName SymbolName) (SymbolValue, error)
}

// Symbol represents the name of a symbol
type Symbol struct {
	Name    SymbolName
	Address SymbolValue
	Size    uint64
}

func (s Symbol) AsRange() util.Range {
	return util.Range{
		Start: uint64(s.Address),
		End:   uint64(s.Address) + s.Size,
	}
}

var _ SymbolFinder = &SymbolMap{}

// SymbolMap represents collections of symbols that can be resolved or reverse mapped
type SymbolMap struct {
	nameToSymbol    map[SymbolName]*Symbol
	addressToSymbol []Symbol
}

func NewSymbolMap(capacity int) *SymbolMap {
	return &SymbolMap{
		addressToSymbol: make([]Symbol, 0, capacity),
	}
}

// Add a symbol to the map
func (symmap *SymbolMap) Add(s Symbol) {
	symmap.addressToSymbol = append(symmap.addressToSymbol, s)
}

// Finalize symbol map by sorting and constructing the nameToSymbol table after
// all symbols are inserted via Add() calls
func (symmap *SymbolMap) Finalize() {
	// Adjust the overcommitted capacity
	a := make([]Symbol, len(symmap.addressToSymbol))
	copy(a, symmap.addressToSymbol)
	symmap.addressToSymbol = a

	sort.Slice(symmap.addressToSymbol,
		func(i, j int) bool {
			return symmap.addressToSymbol[i].Address > symmap.addressToSymbol[j].Address
		})

	symmap.nameToSymbol = make(map[SymbolName]*Symbol, len(symmap.addressToSymbol))
	for i, s := range symmap.addressToSymbol {
		symmap.nameToSymbol[s.Name] = &symmap.addressToSymbol[i]
	}
}

// LookupSymbol obtains symbol information. Returns nil and an error if not found.
func (symmap *SymbolMap) LookupSymbol(symbolName SymbolName) (*Symbol, error) {
	if sym, ok := symmap.nameToSymbol[symbolName]; ok {
		return sym, nil
	}
	return nil, fmt.Errorf("symbol %v not present in map", symbolName)
}

// LookupSymbolsByPrefix loops over all known symbols and returns all symbols
// that start with the given prefix.
func (symmap *SymbolMap) LookupSymbolsByPrefix(prefix string) ([]*Symbol, error) {
	var symbols []*Symbol
	for name, sym := range symmap.nameToSymbol {
		if strings.HasPrefix(string(name), prefix) {
			symbols = append(symbols, sym)
		}
	}
	if len(symbols) == 0 {
		return nil, fmt.Errorf("no symbol present that starts with '%s'", prefix)
	}
	return symbols, nil
}

// LookupSymbolAddress returns the address of a symbol.
// Returns SymbolValueInvalid and error if not found.
func (symmap *SymbolMap) LookupSymbolAddress(symbolName SymbolName) (SymbolValue, error) {
	if sym, ok := symmap.nameToSymbol[symbolName]; ok {
		return sym.Address, nil
	}
	return SymbolValueInvalid, fmt.Errorf("symbol %v not present in map", symbolName)
}

// LookupByAddress translates the address to a symbolic information. Return empty string and
// absolute address if it did not match any symbol.
func (symmap *SymbolMap) LookupByAddress(val SymbolValue) (SymbolName, Address, bool) {
	i := sort.Search(len(symmap.addressToSymbol),
		func(i int) bool {
			return val >= symmap.addressToSymbol[i].Address
		})
	if i < len(symmap.addressToSymbol) &&
		(symmap.addressToSymbol[i].Size == 0 ||
			val < symmap.addressToSymbol[i].Address+
				SymbolValue(symmap.addressToSymbol[i].Size)) {
		return symmap.addressToSymbol[i].Name,
			Address(val - symmap.addressToSymbol[i].Address),
			true
	}
	return SymbolNameUnknown, Address(val), false
}

// VisitAll calls the provided callback with all the symbols in the map.
func (symmap *SymbolMap) VisitAll(cb func(Symbol)) {
	for _, f := range symmap.nameToSymbol {
		cb(*f)
	}
}

// Len returns the number of elements in the map.
func (symmap *SymbolMap) Len() int {
	return len(symmap.addressToSymbol)
}
