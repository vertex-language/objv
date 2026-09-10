// Package lower translates a checked syntax tree into VIR.
//
// It is the last phase that knows what Objective-C is. Everything below it
// sees a typed SSA module: a message send has become a call through a
// selector reference, an instance variable a load of an offset the runtime
// will write, a block literal a structure and a function, a property access
// the accessor send the program did not spell. What arrives here as a tree
// leaves as instructions, and nothing downstream has to ask what `@` meant.
//
// The rule this package works to is that lowering is a *translation*, not a
// second opinion. The analyzer decided what every expression means and
// recorded it in analyzer.Info — which method a send resolves to, which
// property a dot names, what type everything has — and this package reads
// those answers rather than recomputing them. Where it cannot emit something
// it says so, once, with the construct named: a lowering that guesses
// produces a program that compiles and misbehaves, which is worse than one
// that refuses.
package lower

import (
	"fmt"

	"github.com/vertex-language/ir"
	"github.com/vertex-language/objv/analyzer"
	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/runtime"
	"github.com/vertex-language/objv/token"
	"github.com/vertex-language/objv/types"
)

// Options is everything lower needs from its caller that the tree does not
// say.
type Options struct {
	// Name is the module name: a bare identifier, from the primary source
	// file's stem. Empty means "a".
	Name string

	// Target supplies the use path and the ir.Layout the module opens with.
	Target ir.Target

	// Model must be the same model analyzer.Check ran against. sizeof
	// disagreeing between the two is a bug, not a configuration.
	Model types.Model

	// ABI is the runtime ABI: which runtime, which container, how wide a
	// pointer is. It decides every symbol name and section this package
	// writes.
	ABI runtime.ABI

	// Arch is the target architecture, which decides which objc_msgSend a
	// send goes through. Getting it wrong corrupts return values rather
	// than failing to link, which is why it is a field and not a guess.
	Arch runtime.Arch

	// ARC says the unit was analyzed with automatic reference counting on.
	// It must match the mode analyzer.Check ran with: the ownership this
	// package acts on is the ownership that analysis inferred.
	ARC bool

	// SymbolPrefix is what an identifier becomes in an object file: "_" on
	// Mach-O, empty on ELF and COFF.
	//
	// It is stated rather than derived from Target, because the mapping is
	// the language's and not the IR's — nothing below this package renames
	// a symbol. A module built with the wrong prefix compiles and fails to
	// link, naming what it could not find.
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
	arc   bool

	mod    *ir.Module
	target ir.Target

	top   *scope // file scope
	scope *scope // innermost open scope
	fn    *fnState

	// The pools. Each of these is a name the runtime or the linker expects
	// to see once per translation unit however many times the source wrote
	// it: two sends of the same selector share one selector reference, and
	// two mentions of a string share one constant.
	selRefs   map[string]ir.Symbol
	classRefs map[string]ir.Symbol
	superRefs map[string]ir.Symbol
	strs      map[string]ir.Symbol
	cstrs     map[string]ir.Symbol

	// externs are the runtime entry points this unit called, so that each
	// is imported once.
	externs map[string]*ir.FuncImport

	// classSyms are the class and metaclass objects this unit defined or
	// referenced, by symbol name, so that a reference and a definition are
	// one symbol.
	classSyms map[string]ir.Symbol

	// What the metadata pass will need: the classes and categories this
	// unit implemented, the methods it compiled, and the lists the runtime
	// scans.
	impls        []*types.Class
	categories   []categoryImpl
	methodFns    []methodFn
	classList    []ir.Symbol
	categoryList []ir.Symbol

	// defines is the functions this unit defines, so that a declaration of
	// one is not imported alongside it; funcs is the definition itself.
	defines map[string]bool
	funcs   map[string]*ir.Func

	// ivarSyms are the offset variables, by symbol name. A body reads one
	// before the metadata pass writes it, and both need the same symbol.
	ivarSyms map[string]ir.Symbol

	// records are the VIR struct types C records became, by identity. A
	// nil entry is a record VIR cannot describe, remembered so that the
	// same one is not walked twice.
	records map[*types.Record]*ir.Type

	// protoSyms are the protocol objects this unit emitted, by name. A
	// protocol is emitted on first mention and coalesced at link time.
	protoSyms map[string]ir.Symbol

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
		src:       src,
		file:      file,
		info:      info,
		model:     opt.Model,
		abi:       abi,
		arch:      opt.Arch,
		arc:       opt.ARC,
		mod:       ir.NewModule(name, opt.Target),
		target:    opt.Target,
		top:       top,
		scope:     top,
		selRefs:   map[string]ir.Symbol{},
		classRefs: map[string]ir.Symbol{},
		superRefs: map[string]ir.Symbol{},
		strs:      map[string]ir.Symbol{},
		cstrs:     map[string]ir.Symbol{},
		externs:   map[string]*ir.FuncImport{},
		classSyms: map[string]ir.Symbol{},
		defines:   map[string]bool{},
		ivarSyms:  map[string]ir.Symbol{},
		records:   map[*types.Record]*ir.Type{},
		protoSyms: map[string]ir.Symbol{},
		funcs:     map[string]*ir.Func{},
		symPrefix: opt.SymbolPrefix,
		warned:    map[string]bool{},
	}
}

// sym is what an identifier is called in the object file.
//
// It also spells the one Objective-C symbol VIR cannot: an instance
// variable's offset is `OBJC_IVAR_$_Class._name`, and a VIR symbol is an
// identifier — letters, digits, underscore and dollar. The dot becomes a
// dollar, which the ABI's own names already use as a separator and which no
// class or variable name may contain, so the mapping is unambiguous and
// reversible.
//
// It is a limitation of the IR's symbol alphabet rather than a decision:
// nothing about the runtime wants this, and an object writer that emitted
// the dot would be emitting what clang does. The other name in the same
// position — a method's `-[Class selector]` — is handled where it is built,
// by runtime.MethodSymbol.
func (u *unit) sym(name string) string {
	out := make([]byte, 0, len(name)+len(u.symPrefix))
	out = append(out, u.symPrefix...)
	for i := 0; i < len(name); i++ {
		if name[i] == '.' {
			out = append(out, '$')
			continue
		}
		out = append(out, name[i])
	}
	return string(out)
}

func (u *unit) name(id *ast.Ident) string {
	if id == nil {
		return ""
	}
	return id.Name(u.src)
}

// typeOf is the type analysis gave a node. A nil answer means analysis said
// nothing, which happens only on a tree it already reported on.
func (u *unit) typeOf(n ast.Node) types.Type { return u.info.Types[n] }

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
	if f, ok := u.externs[name]; ok {
		return f
	}
	f := u.mod.ImportFunc(u.sym(name), sig)
	u.externs[name] = f
	return f
}
