package lower

import (
	"github.com/vertex-language/ir"
	"github.com/vertex-language/objv/types"
)

// Where a name's storage is.
//
// Every local is a slot: an alloc in the entry block, written and read
// through. Nothing here builds SSA values for locals directly, because a
// variable that is addressed, captured by a block, or assigned in a loop
// needs a place to be — and deciding which locals could have avoided one is
// the optimizer's job, not the front end's.

// storageKind says what a name resolves to.
type storageKind uint8

const (
	// stLocal is a slot in the current frame.
	stLocal storageKind = iota
	// stGlobal is a module-scope datum, reached by its symbol.
	stGlobal
	// stFunc is a function, reached by its symbol.
	stFunc
	// stIvar is an instance variable: an offset the runtime writes, added
	// to self.
	stIvar
	// stEnum is an enumeration constant, which has no storage at all.
	stEnum
)

// storage is what a name means in the block being built.
type storage struct {
	kind storageKind
	typ  types.Type

	// addr is the slot, for a local.
	addr ir.Ptr
	// sym is the symbol, for a global or a function.
	sym ir.Symbol
	// class and ivar name the instance variable, for stIvar.
	class string
	ivar  string
	// value is the constant, for an enumeration constant.
	value int64
}

type scope struct {
	parent *scope
	names  map[string]*storage
}

func newScope(parent *scope) *scope {
	return &scope{parent: parent, names: map[string]*storage{}}
}

func (u *unit) push() { u.scope = newScope(u.scope) }
func (u *unit) pop()  { u.scope = u.scope.parent }

func (u *unit) bind(name string, s *storage) {
	if name != "" {
		u.scope.names[name] = s
	}
}

func (u *unit) lookup(name string) *storage {
	for s := u.scope; s != nil; s = s.parent {
		if st, ok := s.names[name]; ok {
			return st
		}
	}
	return nil
}
