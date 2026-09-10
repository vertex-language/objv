package lower

import (
	"github.com/vertex-language/ir"
	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/runtime"
	"github.com/vertex-language/objv/types"
)

// The metadata: what the runtime walks at load, and what the methods it
// finds there were compiled into.
//
// A class is not code. It is a pair of structures — the class object and its
// metaclass, each pointing at a read-only half — plus a method list, an ivar
// list, a property list, an offset variable per instance variable, and an
// entry in a section the runtime scans. None of that is reachable from any
// call, which is why every list is emitted whether or not the program
// mentions it and why the sections say no_dead_strip.

// defineImpl lowers the method bodies of an @implementation, and records
// what its metadata will need.
func (u *unit) defineImpl(class, category string, members []ast.Decl) {
	k := u.classNamed(class)
	if k == nil {
		return
	}
	for _, m := range members {
		switch m := m.(type) {
		case *ast.MethodDecl:
			u.defineMethod(k, category, m)
		case *ast.FuncDecl:
			u.defineFunc(m)
		case *ast.GenDecl:
			u.defineGlobals(m)
		}
	}
	if category == "" {
		u.impls = append(u.impls, k)
	} else {
		u.categories = append(u.categories, categoryImpl{class: k, name: category})
	}
}

// classNamed finds the analyzed class.
func (u *unit) classNamed(name string) *types.Class {
	for _, k := range u.info.Classes {
		if k.Name == name {
			return k
		}
	}
	return nil
}

// defineMethod emits one method's body as a function.
//
// A method is a function of self and _cmd, and the runtime calls it through
// the pointer in the method list. Its symbol is `-[Class selector]`, which
// no C identifier can collide with — and which is what makes a backtrace
// readable.
func (u *unit) defineMethod(k *types.Class, category string, m *ast.MethodDecl) {
	if m.Body == nil {
		return
	}
	sig := u.methodOf(k, m)
	if sig == nil {
		return
	}
	// The symbol is the mangled one: a VIR symbol is an identifier, and
	// `-[Counter count]` is not. runtime.MethodName is what the method is
	// called; runtime.MethodSymbol is what the function is named.
	name := runtime.MethodSymbol(k.Name, category, sig.Sel, sig.Class)
	fn := u.mod.Func(u.sym(name)).Internal()

	// The two hidden parameters, then the ones the selector named.
	self := types.NewObject(k)
	if sig.Class {
		self = &types.Pointer{Elem: &types.Object{Base: k, Meta: true}}
	}
	ft := &types.Func{Ret: sig.Ret, Proto: true, Variadic: sig.Variadic}
	ft.Params = append(ft.Params,
		types.Param{Name: "self", Type: self},
		types.Param{Name: "_cmd", Type: types.NewSelector()})
	ft.Params = append(ft.Params, sig.Params...)

	names := []*ast.Ident{nil, nil}
	for _, part := range m.Parts {
		names = append(names, part.Name)
	}
	for _, p := range m.Params {
		if p.Decl != nil {
			names = append(names, p.Decl.DeclName())
		} else {
			names = append(names, nil)
		}
	}

	u.methodFns = append(u.methodFns, methodFn{class: k, category: category, sig: sig, fn: fn})
	u.buildBody(fn, ft, names, m.Body, k)
}

// methodOf finds the analyzed signature for a definition.
func (u *unit) methodOf(k *types.Class, m *ast.MethodDecl) *types.Method {
	sel := u.selectorOfDecl(m)
	if sel == "" {
		return nil
	}
	return k.Lookup(sel, m.IsClassMethod())
}

func (u *unit) selectorOfDecl(m *ast.MethodDecl) string {
	if m.Sel != nil {
		return u.name(m.Sel)
	}
	var pieces []string
	for _, p := range m.Parts {
		pieces = append(pieces, u.name(p.Sel))
	}
	return types.Selector(pieces, true)
}

// methodFn pairs a compiled method with the class it belongs to.
type methodFn struct {
	class    *types.Class
	category string
	sig      *types.Method
	fn       *ir.Func
}

type categoryImpl struct {
	class *types.Class
	name  string
}

// emitMetadata writes everything the runtime reads.
func (u *unit) emitMetadata() {
	if len(u.impls) == 0 && len(u.categories) == 0 {
		return
	}
	for _, k := range u.impls {
		u.emitClass(k)
	}
	for _, c := range u.categories {
		u.emitCategory(c)
	}
	u.emitImageInfo()
}

// emitClass writes a class, its metaclass, and everything they point at.
func (u *unit) emitClass(k *types.Class) {
	name := u.classNameString(k.Name)

	instMethods := u.emitMethodList(runtime.InstanceMethodsSymbol(k.Name), u.methodsOf(k, "", false))
	classMethods := u.emitMethodList(runtime.ClassMethodsSymbol(k.Name), u.methodsOf(k, "", true))
	ivars := u.emitIvarList(k)
	props := u.emitPropertyList(runtime.PropertiesSymbol(k.Name), k.Properties, k.Name)

	start, size := u.instanceLayout(k)

	// The metaclass first: its read-only half holds the class methods, and
	// its instance size is the size of a class object.
	metaFlags := runtime.ClassFlags(true, k.Root, u.arc, false, false)
	clsSize := uint32(u.abi.SizeOf(runtime.Class))
	// The conformance list, which is the same object in both halves: a
	// class and its metaclass conform to the same protocols.
	protocols := u.emitProtocolList(runtime.ClassProtocolsSymbol(k.Name), k.Protocols)

	metaRO := u.emitClassRO(runtime.MetaclassROSymbol(k.Name), uint32(metaFlags),
		clsSize, clsSize, name, classMethods, protocols, nil, nil)

	flags := runtime.ClassFlags(false, k.Root, u.arc, false, false)
	ro := u.emitClassRO(runtime.ClassROSymbol(k.Name), uint32(flags),
		start, size, name, instMethods, protocols, ivars, props)

	// A root class's metaclass is its own isa, which is what closes the
	// chain the runtime walks looking for a class method.
	// A root class's metaclass has itself as its isa and the class as its
	// superclass, which is what closes the chain the runtime walks looking
	// for a class method.
	metaName := runtime.MetaclassSymbol(k.Name)
	clsName := runtime.ClassSymbol(k.Name)
	superMeta, superCls := metaName, clsName
	if k.Super != nil {
		superMeta = runtime.MetaclassSymbol(k.Super.Name)
		superCls = runtime.ClassSymbol(k.Super.Name)
	}
	meta := u.emitClassObject(metaName, u.classSymbol(superMeta), u.classSymbol(superCls), metaRO)
	u.emitClassObject(clsName, meta, u.classSymbol(superCls), ro)

	// The list the runtime scans. One entry per class the image defines.
	u.classList = append(u.classList, u.classSyms[clsName])
}

// instanceLayout is what the compiler believed the instance layout was.
//
// The runtime compares instanceStart against the superclass's real size and
// slides every instance variable by the difference. Both numbers are
// therefore a *claim* rather than a fact, and the offset variables are what
// the truth is written into.
func (u *unit) instanceLayout(k *types.Class) (start, size uint32) {
	off := uint64(0)
	if k.Super != nil {
		_, superSize := u.instanceLayout(k.Super)
		off = uint64(superSize)
	} else {
		off = uint64(u.abi.PtrBytes) // the isa
	}
	start = uint32(off)
	for i := range k.Ivars {
		sz, align := u.sizeAlign(k.Ivars[i].Type)
		off = roundUp(off, align)
		off += sz
	}
	return start, uint32(off)
}

func roundUp(n, to uint64) uint64 {
	if to <= 1 {
		return n
	}
	return (n + to - 1) / to * to
}

// emitClassObject writes the five words of a class object.
// declareClass creates the class and metaclass objects of a class this unit
// implements, before anything can reference them.
//
// The initializer is filled in later, by emitClassObject: it names the
// read-only halves, which do not exist until the method and ivar lists have
// been walked. What matters here is only that the *symbol* is a definition —
// a send to the class, a super send, or the class's own subclass all ask for
// it while bodies are being lowered, and whoever asks first decides whether
// this module defines the name or imports it.
func (u *unit) declareClass(k *types.Class) {
	for _, name := range []string{runtime.ClassSymbol(k.Name), runtime.MetaclassSymbol(k.Name)} {
		if _, done := u.classSyms[name]; done {
			continue
		}
		u.classSyms[name] = u.mod.Global(u.sym(name), ir.RW,
			u.metaType("objc_class", runtime.Class).FType()).
			Export().
			Section(u.abi.Name(runtime.SecClassData)).
			Align(uint64(u.abi.PtrBytes))
	}
}

func (u *unit) emitClassObject(name string, isa, super ir.Symbol, ro ir.Symbol) ir.Symbol {
	g, ok := u.classSyms[name].(*ir.Global)
	if !ok {
		g = u.mod.Global(u.sym(name), ir.RW,
			u.metaType("objc_class", runtime.Class).FType()).
			Export().
			Section(u.abi.Name(runtime.SecClassData)).
			Align(uint64(u.abi.PtrBytes))
		u.classSyms[name] = g
	}
	g.Init(ir.List(
		ir.RelocInit(isa),
		ir.RelocInit(super),
		ir.RelocInit(u.emptyCache()),
		ir.Lit(ir.Int(0)),
		ir.RelocInit(ro),
	))
	return g
}

// emitClassRO writes the read-only half.
func (u *unit) emitClassRO(name string, flags, start, size uint32,
	className, methods, protocols, ivars, props ir.Symbol) ir.Symbol {

	fields := []ir.Init{
		ir.Lit(ir.Int(int64(flags))),
		ir.Lit(ir.Int(int64(start))),
		ir.Lit(ir.Int(int64(size))),
		ir.Lit(ir.Int(0)), // the reserved word of a 64-bit target
		ir.Lit(ir.Int(0)), // ivarLayout: no strong or weak map yet
		ir.RelocInit(className),
		orNull(methods),
		orNull(protocols),
		orNull(ivars),
		ir.Lit(ir.Int(0)), // weakIvarLayout
		orNull(props),
	}
	return u.mod.Global(u.sym(name), ir.RO,
		u.metaType("objc_class_ro", runtime.ClassRO).FType()).
		Internal().
		Section(u.abi.Name(runtime.SecConst)).
		Align(uint64(u.abi.PtrBytes)).
		Init(ir.List(fields...))
}

// emitMethodList writes a method list, or nothing when there are no methods.
func (u *unit) emitMethodList(name string, ms []methodFn) ir.Symbol {
	if len(ms) == 0 {
		return nil
	}
	var items []ir.Init
	for _, m := range ms {
		items = append(items, ir.List(
			ir.RelocInit(u.methodName(m.sig.Sel)),
			ir.RelocInit(u.methodTypeString(u.abi.MethodTypes(m.sig.Ret, m.sig.Params, u.model))),
			ir.RelocInit(m.fn)))
	}
	return u.mod.Global(u.sym(name), ir.RO,
		u.listType("objc_method", runtime.Method, len(ms)).FType()).
		Internal().
		Section(u.abi.Name(runtime.SecConst)).
		Align(uint64(u.abi.PtrBytes)).
		Init(u.listInit(u.abi.EntSize(runtime.Method), items))
}

// methodsOf is the methods this unit compiled for a class or a category.
func (u *unit) methodsOf(k *types.Class, category string, class bool) []methodFn {
	var out []methodFn
	for _, m := range u.methodFns {
		if m.class == k && m.category == category && m.sig.Class == class {
			out = append(out, m)
		}
	}
	return out
}

// emitIvarList writes the instance variables, and the offset variable each
// one is reached through.
// ivarOffsets is where each of a class's own instance variables sits,
// counting from the end of the superclass's instances.
//
// It is the compiler's guess and not the answer: a non-fragile runtime
// rewrites every offset variable when it realizes the class, which is the
// whole reason an access loads one instead of adding a constant. The guess
// still has to be right for the superclass this unit compiled against, since
// that is what the runtime starts from.
func (u *unit) ivarOffsets(k *types.Class) []uint64 {
	start, _ := u.instanceLayout(k)
	off := uint64(start)
	out := make([]uint64, len(k.Ivars))
	for i := range k.Ivars {
		size, align := u.sizeAlign(k.Ivars[i].Type)
		off = roundUp(off, align)
		out[i] = off
		off += size
	}
	return out
}

// declareIvars defines the offset variables of a class this unit implements.
//
// It runs before any body is lowered, because a body that reads an instance
// variable asks for the same symbol: whoever gets there first decides whether
// the module defines it or imports it, and for a class defined right here the
// answer has to be "defines".
func (u *unit) declareIvars(k *types.Class) {
	offs := u.ivarOffsets(k)
	for i := range k.Ivars {
		name := runtime.IvarOffsetSymbol(k.Name, k.Ivars[i].Name)
		if _, done := u.ivarSyms[name]; done {
			continue
		}
		u.ivarSyms[name] = u.mod.Global(u.sym(name), ir.RW, ir.StoreI32.FType()).
			Export().
			Section(u.abi.Name(runtime.SecIvarOffsets)).
			Align(4).
			Init(ir.Lit(ir.Int(int64(offs[i]))))
	}
}

func (u *unit) emitIvarList(k *types.Class) ir.Symbol {
	if len(k.Ivars) == 0 {
		return nil
	}
	var items []ir.Init
	for i := range k.Ivars {
		iv := &k.Ivars[i]
		size, align := u.sizeAlign(iv.Type)
		items = append(items, ir.List(
			ir.RelocInit(u.ivarOffsetSymbol(k.Name, iv.Name)),
			ir.RelocInit(u.methodName(iv.Name)),
			ir.RelocInit(u.methodTypeString(u.abi.EncodeExtended(iv.Type, u.model))),
			ir.Lit(ir.Int(int64(log2u(align)))),
			ir.Lit(ir.Int(int64(size)))))
	}
	return u.mod.Global(u.sym(runtime.IvarsSymbol(k.Name)), ir.RO,
		u.listType("objc_ivar", runtime.Ivar, len(k.Ivars)).FType()).
		Internal().
		Section(u.abi.Name(runtime.SecConst)).
		Align(uint64(u.abi.PtrBytes)).
		Init(u.listInit(u.abi.EntSize(runtime.Ivar), items))
}

func log2u(n uint64) uint64 {
	k := uint64(0)
	for n > 1 {
		n >>= 1
		k++
	}
	return k
}

// emitPropertyList writes the properties, as name and attribute string.
func (u *unit) emitPropertyList(name string, props []*types.Property, owner string) ir.Symbol {
	var mine []*types.Property
	for _, p := range props {
		if p.Owner == owner {
			mine = append(mine, p)
		}
	}
	if len(mine) == 0 {
		return nil
	}
	var items []ir.Init
	for _, p := range mine {
		items = append(items, ir.List(
			ir.RelocInit(u.methodName(p.Name)),
			ir.RelocInit(u.methodName(u.propertyAttrs(p)))))
	}
	return u.mod.Global(u.sym(name), ir.RO,
		u.listType("objc_property", runtime.Property, len(mine)).FType()).
		Internal().
		Section(u.abi.Name(runtime.SecConst)).
		Align(uint64(u.abi.PtrBytes)).
		Init(u.listInit(u.abi.EntSize(runtime.Property), items))
}

// propertyAttrs is the attribute string a property carries.
func (u *unit) propertyAttrs(p *types.Property) string {
	desc := runtime.PropertyDesc{
		Type:      u.abi.EncodeExtended(p.Type, u.model),
		Readonly:  p.Has(types.PropReadonly),
		Copy:      p.Has(types.PropCopy),
		Retain:    p.Has(types.PropRetain) || p.Has(types.PropStrong),
		Weak:      p.Has(types.PropWeak),
		Nonatomic: p.Has(types.PropNonatomic),
		Dynamic:   p.Dynamic,
		Ivar:      p.Ivar,
	}
	if p.Has(types.PropGetter) {
		desc.Getter = p.Getter
	}
	if p.Has(types.PropSetter) {
		desc.Setter = p.Setter
	}
	return runtime.PropertyAttributes(desc)
}

// emitCategory writes a category's metadata.
func (u *unit) emitCategory(c categoryImpl) {
	k, name := c.class, c.name
	inst := u.emitMethodList(runtime.CategoryInstanceMethodsSymbol(k.Name, name),
		u.methodsOf(k, name, false))
	cls := u.emitMethodList(runtime.CategoryClassMethodsSymbol(k.Name, name),
		u.methodsOf(k, name, true))

	g := u.mod.Global(u.sym(runtime.CategorySymbol(k.Name, name)), ir.RO,
		u.metaType("objc_category", runtime.Category).FType()).
		Internal().
		Section(u.abi.Name(runtime.SecConst)).
		Align(uint64(u.abi.PtrBytes)).
		Init(ir.List(
			ir.RelocInit(u.classNameString(name)),
			ir.RelocInit(u.classSymbol(runtime.ClassSymbol(k.Name))),
			orNull(inst),
			orNull(cls),
			ir.Lit(ir.Int(0)), // protocols
			ir.Lit(ir.Int(0)), // instance properties
			ir.Lit(ir.Int(0)), // class properties
			ir.Lit(ir.Int(u.abi.SizeOf(runtime.Category))),
			ir.Lit(ir.Int(0)), // the reserved word
		))
	u.categoryList = append(u.categoryList, g)
}

// emitImageInfo writes the two words every image carries, and the lists the
// runtime scans to find this one's work.
func (u *unit) emitImageInfo() {
	if len(u.classList) > 0 {
		u.emitList(runtime.ClassListLabel, runtime.SecClassList, u.classList)
	}
	if len(u.categoryList) > 0 {
		u.emitList(runtime.CategoryListLabel, runtime.SecCategoryList, u.categoryList)
	}
	u.mod.Global(u.sym(runtime.ImageInfoLabel), ir.RO,
		u.metaType("objc_image_info", runtime.ImageInfo).FType()).
		Internal().
		Section(u.abi.Name(runtime.SecImageInfo)).
		Align(4).
		Init(ir.List(
			ir.Lit(ir.Int(0)),
			ir.Lit(ir.Int(int64(runtime.ImageInfoFlags()))),
		))
}

// emitList writes one of the pointer lists the runtime walks.
func (u *unit) emitList(name string, sec runtime.Section, syms []ir.Symbol) {
	items := make([]ir.Init, 0, len(syms))
	for _, s := range syms {
		items = append(items, ir.RelocInit(s))
	}
	u.mod.Global(u.sym(name), ir.RW, ir.Array(uint64(len(syms)), u.ptrFType())).
		Internal().
		Section(u.abi.Name(sec)).
		Align(uint64(u.abi.PtrBytes)).
		Init(ir.List(items...))
}

// emptyCache is the shared empty cache every class points at until the
// runtime gives it one.
func (u *unit) emptyCache() ir.Symbol {
	if s := u.mod.Lookup(u.sym(runtime.EmptyCache)); s != nil {
		return s
	}
	return u.mod.ImportGlobal(u.sym(runtime.EmptyCache), u.ptrFType())
}

// orNull is a pointer field that may have nothing in it.
func orNull(s ir.Symbol) ir.Init {
	if s == nil {
		return ir.Lit(ir.Int(0))
	}
	return ir.RelocInit(s)
}
