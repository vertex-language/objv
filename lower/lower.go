// Package lower translates a checked AST into VIR (Vertex Intermediate Representation).
//
// It translates Objective-C constructs (message sends, ivars, block literals, properties)
// into typed SSA instructions using semantic answers recorded in analyzer.Info.
package lower

import (
	"fmt"
	"strings"

	"github.com/vertex-language/ir"
	"github.com/vertex-language/objv/analyzer"
	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/runtime"
	"github.com/vertex-language/objv/token"
	"github.com/vertex-language/objv/types"
)

// Options specifies target, ABI, and compiler mode flags for lowering.
type Options struct {
	// Name is the module name (stem of primary source file; defaults to "a").
	Name string

	// Target supplies the target layout and use path.
	Target ir.Target

	// Model must match the types.Model used during analysis.
	Model types.Model

	// ABI specifies the runtime ABI and container format.
	ABI runtime.ABI

	// Arch is the target architecture for selecting msgSend variants.
	Arch runtime.Arch

	// Platform and Deployment are used for @available checks.
	Platform   runtime.Platform
	Deployment runtime.OSVersion

	// ARC indicates whether automatic reference counting is enabled.
	ARC bool

	// SymbolPrefix is the symbol prefix ("_" on Mach-O, empty on ELF/COFF).
	SymbolPrefix string
}

// Lower emits one translation unit as a module.
//
// The module is never nil, even when diagnostics are returned: a partial
// module is what `objv build --emit vir` on broken input should print.
// Diagnostics are sorted, and a sticky builder failure inside ir surfaces as
// exactly one of them — every call after the first is a no-op, so reporting
// each would be a cascade.
func Lower(unit *token.File, file *ast.File, info *analyzer.Info, opt Options) (*ir.Module, []token.Diagnostic) {
	u := newUnit(unit, file, info, opt)
	u.declareFile()
	u.defineFile()
	u.emitMetadata()
	u.emitStaticInit()
	if err := u.mod.Err(); err != nil {
		u.errorf(file, "internal: the IR builder rejected this unit: %v", err)
	}
	token.SortDiagnostics(u.diags)
	return u.mod, u.diags
}

// unit is one translation unit's lowering state.
type unit struct {
	src   *token.File
	file  *ast.File
	info  *analyzer.Info
	model types.Model
	abi   runtime.ABI
	arch  runtime.Arch

	platform   runtime.Platform
	deployment runtime.OSVersion
	arc        bool

	mod    *ir.Module
	target ir.Target

	top   *scope // file scope
	scope *scope // innermost open scope
	fn    *fnState

	// Reference and string pools
	selRefs   map[string]ir.Symbol
	classRefs map[string]ir.Symbol
	superRefs map[string]ir.Symbol
	strs      map[string]ir.Symbol
	cstrs     map[string]ir.Symbol

	// externs tracks imported runtime entry points.
	externs map[string]*ir.FuncImport

	// classSyms tracks class and metaclass symbols.
	classSyms map[string]ir.Symbol

	// sigs caches parameter lists for functions.
	sigs map[*ir.Func]funcSig

	// funcNameText is the value of __func__ for the current function.
	funcNameText string

	// classMethod indicates if currently lowering a class method (+).
	classMethod bool

	// ehTypes tracks generated type-info symbols for caught exception types.
	ehTypes map[string]ir.Symbol

	// Metadata collection
	impls               []*types.Class
	categories          []categoryImpl
	methodFns           []methodFn
	classList           []ir.Symbol
	categoryList        []ir.Symbol
	nonLazyClassList    []ir.Symbol
	nonLazyCategoryList []ir.Symbol

	// staticInit, ctors, dtors track module constructor/destructor functions.
	staticInit   map[string]staticInitAttrs
	ctors, dtors []initEntry

	defines    map[string]bool
	funcs      map[string]*ir.Func
	definesVar map[string]bool
	omitted    map[string]bool

	// deallocating indicates whether currently lowering -dealloc.
	deallocating bool

	// hasCxxDestruct tracks classes with a synthesized .cxx_destruct.
	hasCxxDestruct map[*types.Class]bool

	// byrefHelpers caches copy/dispose helper functions for __block variables.
	byrefHelpers map[string]ir.Symbol

	// blockBase and blockSeq generate unique symbol names for block literals.
	blockBase string
	blockSeq  int

	// undescribed stores error messages for unsupported file-scope types, reported on use.
	undescribed map[string]string

	// ivarSyms maps ivar offset symbol names to their symbols.
	ivarSyms map[string]ir.Symbol

	// records caches lowered VIR struct types for C records.
	records map[*types.Record]*ir.Type

	// protoSyms tracks emitted protocol symbols.
	protoSyms map[string]ir.Symbol

	// used tracks conditionally-emitted static/inline definitions.
	used useSet

	symPrefix string
	warned    map[string]bool
	anon      int

	diags []token.Diagnostic
}

func newUnit(src *token.File, file *ast.File, info *analyzer.Info, opt Options) *unit {
	name := opt.Name
	if name == "" {
		name = "a"
	}
	abi := opt.ABI

	if abi.PtrBytes == 0 {
		abi.PtrBytes = int64(opt.Target.Layout().PtrBits / 8)
	}
	top := newScope(nil)
	return &unit{
		src:        src,
		file:       file,
		info:       info,
		model:      opt.Model,
		abi:        abi,
		platform:   opt.Platform,
		deployment: opt.Deployment,
		arch:       opt.Arch,
		arc:        opt.ARC,
		mod:        ir.NewModule(name, opt.Target),
		target:     opt.Target,
		top:        top,
		scope:      top,
		selRefs:    map[string]ir.Symbol{},
		classRefs:  map[string]ir.Symbol{},
		superRefs:  map[string]ir.Symbol{},
		strs:       map[string]ir.Symbol{},
		cstrs:      map[string]ir.Symbol{},
		externs:    map[string]*ir.FuncImport{},
		classSyms:  map[string]ir.Symbol{},
		sigs:       map[*ir.Func]funcSig{},
		ehTypes:    map[string]ir.Symbol{},
		defines:    map[string]bool{},
		definesVar: map[string]bool{},
		omitted:    map[string]bool{},

		byrefHelpers:   map[string]ir.Symbol{},
		hasCxxDestruct: map[*types.Class]bool{},

		undescribed: map[string]string{},
		ivarSyms:    map[string]ir.Symbol{},
		records:     map[*types.Record]*ir.Type{},
		protoSyms:   map[string]ir.Symbol{},
		funcs:       map[string]*ir.Func{},
		symPrefix:   opt.SymbolPrefix,
		warned:      map[string]bool{},
	}
}

// sym is what an identifier is called in the object file: the platform's
// prefix and the name, unchanged.
//
// Unchanged is the point. An instance variable's offset is
// `OBJC_IVAR_$_Class._name` with a dot in it, and objv used to write a
// dollar there because a VIR symbol had the alphabet of an identifier. The
// result was self-consistent and wrong: every reference objv emitted matched
// every definition objv emitted, and neither matched the ones clang wrote —
// so a class compiled by one compiler and subclassed by the other did not
// link. A VIR symbol now carries the dot.
func (u *unit) sym(name string) string { return u.symPrefix + name }

func (u *unit) name(id *ast.Ident) string {
	if id == nil {
		return ""
	}
	return id.Name(u.src)
}

// typeOf is the type analysis gave a node. A nil answer means analysis said
// nothing, which happens only on a tree it already reported on.
// typeOf is the type analysis recorded for a node.
//
// A miss is a lower bug or an analyzer bug, never user error: every node this
// is asked about is one Info documents itself as covering. It reports and
// yields int so that emission continues and the reader sees one diagnostic
// naming the construct, rather than a nil dereference that takes the process
// down and names nothing.
func (u *unit) typeOf(n ast.Node) types.Type {
	if t, ok := u.info.Types[n]; ok && t != nil {
		return t
	}
	u.errorf(n, "internal: no type recorded for %T", n)
	return types.Typ(types.Int)
}

func (u *unit) errorf(n ast.Node, format string, a ...any) {
	pos, end := token.NoPos, token.NoPos
	if n != nil {
		pos, end = n.Pos(), n.End()
	}
	u.diags = append(u.diags, token.Diagnostic{
		Pos: pos, End: end, Severity: token.Error,
		Message: fmt.Sprintf(format, a...),
	})
}

// unsupported reports a construct this package does not emit yet, once per
// kind of construct.
//
// It is an error rather than a warning, and it names what it could not do.
// The alternative is emitting something plausible, which produces a program
// that builds and does the wrong thing — the one outcome a compiler must
// never choose.
func (u *unit) unsupported(n ast.Node, what string) {
	if u.warned[what] {
		return
	}
	u.warned[what] = true
	u.errorf(n, "objv does not lower %s yet", what)
}

// internal reports something the analyzer accepted and this package could
// not lower.
//
// A nil where a value belongs is the one failure that must not be quiet: the
// statement holding it is dropped, and a program that builds with a statement
// missing is worse than one that does not build. Lowering runs only over a
// tree the analyzer accepted, so reaching here is a gap in this package
// rather than a fault in the program.
//
// Nothing is said once something else has been: whatever was reported first
// is the cause, and this would be its echo.
func (u *unit) internal(n ast.Node, what string) {
	for _, d := range u.diags {
		if d.Severity == token.Error {
			return
		}
	}
	u.errorf(n, "internal: %s did not lower", what)
}

// uniq returns a fresh name for something the source did not name.
func (u *unit) uniq(prefix string) string {
	u.anon++
	return fmt.Sprintf("%s.%d", prefix, u.anon)
}

// ---- the runtime's entry points ----

// extern imports a runtime function once and returns it.
//
// The signature is stated at each call because the IR wants one, and because
// the ones that matter — objc_msgSend and its variants — have no single
// signature: a send is called with the arguments the method takes, and the
// trampoline forwards whatever it was given.
func (u *unit) extern(name string, sig *ir.Sig) *ir.FuncImport {
	return u.importFunc(u.sym(name), sig)
}

// importFunc is the one import of a function symbol, however many ways the
// unit reaches it. The program can declare what objv also calls --
// <objc/message.h> declares objc_msgSend -- and the module holds one symbol
// of a name: both are calls through its address with the signature stated
// at the site, so whichever signature the import was made with, it serves.
func (u *unit) importFunc(sym string, sig *ir.Sig) *ir.FuncImport {
	if f, ok := u.externs[sym]; ok {
		return f
	}
	f := u.mod.ImportFunc(sym, sig)
	if returnsTwice[strings.TrimPrefix(sym, u.symPrefix)] {
		f.ReturnsTwice()
	}
	u.externs[sym] = f
	return f
}

// returnsTwice names the C library's functions that return a second time --
// through longjmp, or in a child after vfork. A caller's locals must stay in
// memory across a call to one: the second return restores registers to what
// they held at the first, and a value kept in a register instead of its slot
// comes back as it was. clang knows these names too; the IR, told, keeps
// every slot of such a function where it is.
var returnsTwice = map[string]bool{
	"setjmp": true, "_setjmp": true, "sigsetjmp": true, "__sigsetjmp": true,
	"savectx": true, "vfork": true, "getcontext": true,
}
