package lower

import (
	"sort"
	"strconv"

	"github.com/vertex-language/ir"

	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/runtime"
)

// The functions that run around main.
//
// `__attribute__((constructor))` is not in C. It is in every C program that
// registers something with a library before main gets a chance to, and it is
// how a unit hooks itself into a program that never names it — which is
// exactly why ignoring one is the worst thing this package could do with it.
// A constructor that never runs leaves the registration undone and the
// program looking for a table entry nobody made, a long way from the
// attribute that was dropped.
//
// The two halves are asymmetric, because the platform is:
//
//   - A constructor is a pointer in __DATA,__mod_init_func. dyld calls what
//     is there before main, in the order it is written, so the list is
//     sorted by priority (lower first, 65535 when unwritten) and by source
//     order within a priority.
//
//   - A destructor is a registration. Nothing walks a list of them at exit,
//     so each one is handed to __cxa_atexit from inside a constructor this
//     package synthesizes, and the C runtime runs them in reverse order of
//     registration on the way out. clang does the same thing and gives the
//     synthesized function the same name; the third argument is the image's
//     __dso_handle, which is what makes unloading a bundle run its
//     destructors and nobody else's.
//
// The priority is a GNU extension on a GNU extension. Darwin's linker does
// not sort the section, so the sort has to happen here — which means it
// orders this unit's constructors against each other and says nothing about
// another unit's. That is the same guarantee clang gives.

// defaultInitPriority is what a constructor without one gets. GCC reserves
// everything below 101 for the implementation, and 65535 is the documented
// default; clang writes the same number into the name of the function it
// synthesizes for destructors.
const defaultInitPriority = 65535

// initEntry is one pointer that will go in __mod_init_func.
type initEntry struct {
	sym  ir.Symbol
	prio int
	seq  int // source order, to break a tie the way the source did
}

// staticInitAttrs is what the attributes on a declaration of a function said
// about running it around main: a priority, or nothing.
type staticInitAttrs struct {
	ctor, dtor bool
	ctorPrio   int
	dtorPrio   int
}

// planStaticInit reads every declaration in the unit for constructor and
// destructor attributes, by name.
//
// It is a pass over declarations rather than a look at the definition,
// because the attribute may be nowhere near the body:
//
//	static void shutdown(void) __attribute__((destructor));
//	…
//	static void shutdown(void) { … }
//
// is how the attribute is usually written when the function is also called
// by name, and clang honours it. A declaration is a promise about the
// function, and this is one of the things it can promise.
func (u *unit) planStaticInit() map[string]staticInitAttrs {
	if u.staticInit != nil {
		return u.staticInit
	}
	u.staticInit = map[string]staticInitAttrs{}
	note := func(name string, attrs ...[]*ast.Attr) {
		if name == "" {
			return
		}
		got := u.staticInit[name]
		for _, list := range attrs {
			for _, a := range list {
				if a == nil || a.Name == nil {
					continue
				}
				switch trimAttrName(u.name(a.Name)) {
				case "constructor":
					got.ctor, got.ctorPrio = true, u.attrPriority(a)
				case "destructor":
					got.dtor, got.dtorPrio = true, u.attrPriority(a)
				}
			}
		}
		if got.ctor || got.dtor {
			u.staticInit[name] = got
		}
	}
	for _, d := range u.file.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			note(u.name(d.Name), attrsOfSpecs(d.Specs), d.Attrs)
		case *ast.GenDecl:
			specs := attrsOfSpecs(d.Specs)
			for _, it := range d.List {
				if it == nil || it.Decl == nil {
					continue
				}
				// Only a function. `int x __attribute__((constructor));`
				// is not one, and clang rejects it rather than running it.
				if outermostFunc(it.Decl) == nil {
					continue
				}
				note(u.name(it.Decl.DeclName()), specs, it.Attrs)
			}
		}
	}
	return u.staticInit
}

// attrsOfSpecs is the attributes a declaration-specifier list carried.
func attrsOfSpecs(specs ast.DeclSpecs) []*ast.Attr {
	var out []*ast.Attr
	for _, s := range specs {
		if a, ok := s.(*ast.AttrSpec); ok {
			out = append(out, a.Attrs...)
		}
	}
	return out
}

// attrPriority is the number in `constructor(101)`, or the default.
func (u *unit) attrPriority(a *ast.Attr) int {
	if a.Args == nil {
		return defaultInitPriority
	}
	for _, t := range a.Args.List {
		n, err := strconv.Atoi(string(u.src.Slice(t.Pos, t.End)))
		if err == nil {
			return n
		}
	}
	return defaultInitPriority
}

// noteStaticInit records a definition this unit emitted whose declaration
// asked for it to run around main.
func (u *unit) noteStaticInit(name string, fn *ir.Func) {
	got, ok := u.planStaticInit()[name]
	if !ok {
		return
	}
	if got.ctor {
		u.ctors = append(u.ctors, initEntry{sym: fn, prio: got.ctorPrio, seq: len(u.ctors)})
	}
	if got.dtor {
		u.dtors = append(u.dtors, initEntry{sym: fn, prio: got.dtorPrio, seq: len(u.dtors)})
	}
}

// emitStaticInit writes the initializer list, and the function that
// registers the destructors if there are any.
func (u *unit) emitStaticInit() {
	if len(u.dtors) > 0 {
		if fn := u.emitDtorRegistration(); fn != nil {
			u.ctors = append(u.ctors, initEntry{
				sym: fn, prio: defaultInitPriority, seq: len(u.ctors),
			})
		}
	}
	if len(u.ctors) == 0 {
		return
	}
	sort.SliceStable(u.ctors, func(i, j int) bool {
		if u.ctors[i].prio != u.ctors[j].prio {
			return u.ctors[i].prio < u.ctors[j].prio
		}
		return u.ctors[i].seq < u.ctors[j].seq
	})
	items := make([]ir.Init, 0, len(u.ctors))
	for _, c := range u.ctors {
		items = append(items, ir.RelocInit(c.sym))
	}
	u.mod.Global(u.sym(u.uniq("init_funcs")), ir.RW,
		ir.Array(uint64(len(items)), u.ptrFType())).
		Internal().
		Section(u.abi.Name(runtime.SecModInitFunc)).
		Align(uint64(u.abi.PtrBytes)).
		Init(ir.List(items...))
}

// emitDtorRegistration is the function clang calls __GLOBAL_init_65535: one
// __cxa_atexit call per destructor, in source order, so that the runtime
// unwinds them in reverse.
func (u *unit) emitDtorRegistration() *ir.Func {
	name := runtime.StaticInitFunc + strconv.Itoa(defaultInitPriority)
	fn := u.mod.Func(u.sym(name)).Internal()
	if sec := u.abi.Name(runtime.SecStaticInit); sec != "" {
		fn.Section(sec)
	}

	leave := u.enterAccessor(fn, nil, nil)
	defer leave()

	sig := ir.NewSig()
	sig.Param(ir.TypePtr).Param(ir.TypePtr).Param(ir.TypePtr).Ret(ir.TypeI32)
	atexit := u.extern(runtime.CxaAtexit, sig)
	handle := u.dsoHandle()

	for _, d := range u.dtors {
		b := u.fn.cur
		b.Call(atexit,
			b.Ptr.GetAddr(d.sym),
			b.Ptr.Const(),
			b.Ptr.GetAddr(handle))
	}
	u.fn.cur.Return()
	return fn
}

// dsoHandle is the image's identity, which the C runtime defines: the main
// executable's comes from crt1.o and a dylib's from the linker.
func (u *unit) dsoHandle() ir.Symbol {
	if s := u.mod.Lookup(u.sym(runtime.DsoHandle)); s != nil {
		return s
	}
	return u.mod.ImportGlobal(u.sym(runtime.DsoHandle), u.ptrFType())
}
