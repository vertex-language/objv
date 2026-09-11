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
	// stByref is a __block variable: it lives in a structure of its own,
	// and every access goes through that structure's forwarding field. See
	// byref.go.
	stByref
)

// storage is what a name means in the block being built.
type storage struct {
	kind storageKind
	typ  types.Type

	// addr is the slot, for a local.
	addr ir.Ptr
	// sym is the symbol, for a global or a function.
	sym ir.Symbol
	// imp is a declaration this unit has read and nothing has used yet.
	// See unit.symOf.
	imp *pendingImport
	// byref is where the variable sits, for stByref. addr is the structure
	// and this says where inside it to look.
	byref *byref
	// class and ivar name the instance variable, for stIvar.
	class string
	ivar  string
	// value is the constant, for an enumeration constant.
	value int64
	// vla is how big it is, for a variably modified array. Its size is not
	// in its type — nothing in types.Array holds an expression — so it is
	// the byte count computed where the declaration stood. See vla.go.
	vla *vlaInfo
}

// pendingImport is an imported symbol that has not been created.
//
// An import is a demand on the linker and a declaration is not. One
// `#import <Foundation/Foundation.h>` reads some nine thousand prototypes
// and the program calls a handful; an object file carrying an undefined
// symbol for each of the rest asks ld for frameworks the program never
// mentioned, and the first thing that fails on is AEBuildAppleEvent, from
// AppleEvents, which arrives through Foundation's include graph and which
// nothing here ever wanted.
//
// Exactly one of sig and ftyp is set: a function has a signature, an object
// has a type.
type pendingImport struct {
	sym  string // the linker name, prefix already applied
	sig  *ir.Sig
	ftyp ir.FType
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

// globalsVisible collects every name in scope that lives outside the frame:
// a static local, a file-scope object, a function, an enumeration constant.
//
// A block body sees file scope and its own names and not the enclosing
// function's locals — what it reached for is in its captures. But a `static`
// local is not a local: §6.2.4 gives it static storage duration, it lives
// where a global lives, and a block that names one is naming a global. Its
// *binding*, though, is in the function's scope, because that is where the
// declaration was written. Dropping the scope drops the binding, and the
// name then resolves to whatever file scope has under it — which for
// `static NSMutableArray *log;` is log() from <math.h>, and the program
// stores an object through a function pointer into the shared cache.
func (u *unit) globalsVisible() map[string]*storage {
	out := map[string]*storage{}
	var walk func(s *scope)
	walk = func(s *scope) {
		if s == nil || s == u.top {
			return
		}
		walk(s.parent) // outermost first, so an inner name wins
		for name, st := range s.names {
			switch st.kind {
			case stGlobal, stFunc, stEnum:
				out[name] = st
			}
		}
	}
	walk(u.scope)
	return out
}

func (u *unit) lookup(name string) *storage {
	for s := u.scope; s != nil; s = s.parent {
		if st, ok := s.names[name]; ok {
			return st
		}
	}
	return nil
}

// symOf is a declared name's symbol, creating the import the first time
// something asks for one. A name that is only declared never reaches the
// module, and so never reaches the object file's symbol table.
func (u *unit) symOf(st *storage) ir.Symbol {
	if st == nil {
		return nil
	}
	if st.sym == nil && st.imp != nil {
		switch {
		case st.imp.sig != nil:
			st.sym = u.mod.ImportFunc(st.imp.sym, st.imp.sig)
		default:
			st.sym = u.mod.ImportGlobal(st.imp.sym, st.imp.ftyp)
		}
		st.imp = nil
	}
	return st.sym
}
