package lower

import (
	"github.com/vertex-language/ir"
	"github.com/vertex-language/objv/runtime"
	"github.com/vertex-language/objv/types"
)

// Protocol metadata.
//
// A protocol is not code and not a class: it is one structure, emitted by
// every image that mentions it and coalesced by the linker down to one. That
// is why every symbol here is weak and hidden — there is no single image that
// owns a protocol, so each states its own copy and the linker picks.
//
// Its method lists carry null implementations, which is the whole point: what
// a protocol declares is a name and a signature, and the extendedMethodTypes
// array beside them is where the runtime reads the encodings back from.

// protocolSym is a protocol's object, emitted once per unit on first use.
func (u *unit) protocolSym(p *types.Protocol) ir.Symbol {
	if p == nil {
		return nil
	}
	if s, ok := u.protoSyms[p.Name]; ok {
		return s
	}
	// Recorded before the body is built: a protocol may inherit from one
	// that inherits from it only through a cycle the analyzer rejects, but
	// the reference below is taken while this one is still being written.
	g := u.mod.Global(u.sym(runtime.ProtocolSymbol(p.Name)), ir.RW,
		u.metaType("objc_protocol", runtime.Protocol).FType()).
		Export().Hidden().Weak().
		Section(u.abi.Name(runtime.SecProtocolData)). // "": the default data section
		Align(uint64(u.abi.PtrBytes))
	u.protoSyms[p.Name] = g

	inst, optInst := splitOptional(p.Methods, false)
	cls, optCls := splitOptional(p.Methods, true)

	name := p.Name
	g.Init(ir.List(
		ir.Lit(ir.Int(0)), // isa: null in the compiled image
		ir.RelocInit(u.classNameString(name)),
		orNull(u.emitProtocolRefs(p)),
		orNull(u.emitProtocolMethods(runtime.ProtocolInstanceMethodsSymbol(name), inst)),
		orNull(u.emitProtocolMethods(runtime.ProtocolClassMethodsSymbol(name), cls)),
		orNull(u.emitProtocolMethods(runtime.ProtocolOptionalInstanceMethodsSymbol(name), optInst)),
		orNull(u.emitProtocolMethods(runtime.ProtocolOptionalClassMethodsSymbol(name), optCls)),
		orNull(u.emitPropertyList(runtime.ProtocolPropertiesSymbol(name), p.Properties, name)),
		ir.Lit(ir.Int(u.abi.SizeOf(runtime.Protocol))),
		ir.Lit(ir.Int(0)), // flags
		orNull(u.emitMethodTypes(p)),
		ir.Lit(ir.Int(0)), // demangledName
		ir.Lit(ir.Int(0)), // classProperties
	))

	// The entry the runtime finds it by. It is a separate symbol from the
	// object because the list is a list of pointers, and because the object
	// itself lives in a section the runtime does not scan.
	u.mod.Global(u.sym(runtime.ProtocolLabelSymbol(name)), ir.RW, u.ptrFType()).
		Export().Hidden().Weak().
		Section(u.abi.Name(runtime.SecProtocolList)).
		Align(uint64(u.abi.PtrBytes)).
		Init(ir.RelocInit(g))
	return g
}

// splitOptional partitions a protocol's methods of one kind into the
// required ones and the @optional ones.
func splitOptional(ms []*types.Method, class bool) (req, opt []*types.Method) {
	for _, m := range ms {
		if m.Class != class {
			continue
		}
		if m.Optional {
			opt = append(opt, m)
			continue
		}
		req = append(req, m)
	}
	return req, opt
}

// emitProtocolMethods writes one of a protocol's four method lists. Each
// entry's implementation is null: a protocol declares and does not define.
func (u *unit) emitProtocolMethods(name string, ms []*types.Method) ir.Symbol {
	if len(ms) == 0 {
		return nil
	}
	items := make([]ir.Init, 0, len(ms))
	for _, m := range ms {
		items = append(items, ir.List(
			ir.RelocInit(u.methodName(m.Sel)),
			ir.RelocInit(u.methodTypeString(u.abi.MethodTypes(m.Ret, m.Params, u.model))),
			ir.Lit(ir.Int(0))))
	}
	return u.mod.Global(u.sym(name), ir.RO,
		u.listType("objc_method", runtime.Method, len(ms)).FType()).
		Internal().
		Section(u.abi.Name(runtime.SecConst)).
		Align(uint64(u.abi.PtrBytes)).
		Init(u.listInit(u.abi.EntSize(runtime.Method), items))
}

// emitMethodTypes is the extendedMethodTypes array: one encoding per method,
// in the order the runtime walks the four lists — required instance,
// required class, optional instance, optional class.
func (u *unit) emitMethodTypes(p *types.Protocol) ir.Symbol {
	inst, optInst := splitOptional(p.Methods, false)
	cls, optCls := splitOptional(p.Methods, true)
	var items []ir.Init
	for _, group := range [][]*types.Method{inst, cls, optInst, optCls} {
		for _, m := range group {
			items = append(items, ir.RelocInit(
				u.methodTypeString(u.abi.MethodTypes(m.Ret, m.Params, u.model))))
		}
	}
	if len(items) == 0 {
		return nil
	}
	return u.mod.Global(u.sym(runtime.ProtocolMethodTypesSymbol(p.Name)), ir.RO,
		ir.Array(uint64(len(items)), u.ptrFType())).
		Internal().
		Section(u.abi.Name(runtime.SecConst)).
		Align(uint64(u.abi.PtrBytes)).
		Init(ir.List(items...))
}

// emitProtocolRefs is the list of protocols a protocol inherits.
func (u *unit) emitProtocolRefs(p *types.Protocol) ir.Symbol {
	if len(p.Inherited) == 0 {
		return nil
	}
	return u.emitProtocolList(runtime.ProtocolRefsSymbol(p.Name), p.Inherited)
}

// emitProtocolList writes a null-terminated protocol list: a pointer-width
// count, the entries, and a null. The shape is the runtime's and not the one
// every other list here uses.
func (u *unit) emitProtocolList(name string, ps []*types.Protocol) ir.Symbol {
	if len(ps) == 0 {
		return nil
	}
	items := []ir.Init{ir.Lit(ir.Int(int64(len(ps))))}
	for _, p := range ps {
		items = append(items, ir.RelocInit(u.protocolSym(p)))
	}
	items = append(items, ir.Lit(ir.Int(0)))
	return u.mod.Global(u.sym(name), ir.RO,
		ir.Array(uint64(len(items)), u.ptrFType())).
		Internal().
		Section(u.abi.Name(runtime.SecConst)).
		Align(uint64(u.abi.PtrBytes)).
		Init(ir.List(items...))
}

// protocolNamed finds an analyzed protocol by name.
func (u *unit) protocolNamed(name string) *types.Protocol {
	for _, p := range u.info.Protocols {
		if p.Name == name {
			return p
		}
	}
	return nil
}
