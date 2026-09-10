package lower

import (
	"strconv"

	"github.com/vertex-language/ir"
	"github.com/vertex-language/objv/runtime"
)

// The metadata structures, as types the module declares.
//
// runtime describes each one as a field list, which is what a reader can
// check against objc4's headers. VIR wants a named struct: an initializer is
// verified against the declared type, so an array of bytes with a list of
// pointers poured into it is not a module — it is a module that happens to
// print. Naming the structures is also what makes the emitted text readable
// as what it is.

// metaType interns a metadata structure as a named struct type.
func (u *unit) metaType(name string, fields []runtime.Field) *ir.Type {
	if t := u.mod.LookupType(name); t != nil {
		return t
	}
	t := u.mod.Struct(name).Internal()
	for i, f := range fields {
		fname := f.Name
		if fname == "" {
			fname = "f" + strconv.Itoa(i)
		}
		t.Field(fname, u.fieldType(f))
	}
	return t
}

// fieldType is the store type one metadata field occupies. Padding is a
// four-byte hole on the targets these layouts describe, and is named rather
// than skipped: a structure written without it is one field short from there
// on, which is the bug this makes impossible to write.
func (u *unit) fieldType(f runtime.Field) ir.FType {
	switch f.Kind {
	case runtime.U32, runtime.Pad:
		return ir.StoreI32.FType()
	case runtime.U64:
		return ir.StoreI64.FType()
	}
	return u.ptrFType()
}

// listType interns the type of an entsize-prefixed list of n entries.
//
// The count is part of the type because the entries are inline: a method
// list of three methods is a different object from one of four, and VIR has
// no flexible array member to spell the difference away.
func (u *unit) listType(name string, entry []runtime.Field, n int) *ir.Type {
	full := name + "_" + strconv.Itoa(n)
	if t := u.mod.LookupType(full); t != nil {
		return t
	}
	et := u.metaType(name, entry)
	t := u.mod.Struct(full).Internal()
	for _, f := range runtime.ListHeader {
		t.Field(f.Name, u.fieldType(f))
	}
	t.Field("entries", ir.Array(uint64(n), et.FType()))
	return t
}

// listInit pairs the header with the entries listType declared.
func (u *unit) listInit(entSize int64, entries []ir.Init) ir.Init {
	return ir.List(
		ir.Lit(ir.Int(entSize)),
		ir.Lit(ir.Int(int64(len(entries)))),
		ir.List(entries...))
}
