// Package analyzer performs semantic analysis: symbol resolution, type checking,
// class hierarchy validation, method and property lookup, and ARC verification.
//
// Analysis runs in two passes:
//
//	Pass 1: Collect declarations (@interface, @protocol, @class, categories).
//	Pass 2: Check definitions, function bodies, and statements in written order.
//
// Results are stored in Info for consumption by the lowering phase.
package analyzer

import (
	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/token"
	"github.com/vertex-language/objv/types"
)

// Mode controls optional analysis behavior.
type Mode uint

const (
	// ARC enables automatic reference counting analysis.
	ARC Mode = 1 << iota
)

// Info records semantic information about an AST.
type Info struct {
	// Types maps AST nodes to their declared or computed types.
	Types map[ast.Node]types.Type

	// Aligns is the alignment a declaration asked for with _Alignas or
	// __attribute__((aligned)), by declarator, where it asked for one. The
	// object is aligned to the larger of this and its type's.
	Aligns map[ast.Node]int64

	// Consts maps evaluated integer constant expressions to their values.
	Consts map[ast.Expr]int64

	// Enums maps enumerators to their integer values.
	Enums map[*ast.Enumerator]int64

	// Sends maps message expressions to resolved methods (or nil for id receivers).
	Sends map[*ast.MessageExpr]*types.Method

	// Props maps property accesses to resolved properties.
	Props map[*ast.MemberExpr]*types.Property

	// Generics maps _Generic expressions to selected association expressions.
	Generics map[*ast.GenericExpr]ast.Expr

	// Overloads maps call identifiers to chosen overload declarations.
	Overloads     map[*ast.Ident]ast.Node
	OverloadIndex map[ast.Node]int

	// Captures maps block literals to their captured variables in first-seen order.
	Captures map[*ast.BlockLit][]Capture

	// Classes, Protocols, and Selectors in order of declaration/appearance.
	Classes   []*types.Class
	Protocols []*types.Protocol
	Selectors []string
}

// Capture represents a variable captured by a block literal.
type Capture struct {
	Name string
	Type types.Type

	// Block is true if declared with __block (shared by reference).
	Block bool
}

// Check analyzes one translation unit against a target model.
//
// The Info is never nil; diagnostics are sorted and each mistake is reported
// once.
func Check(unit *token.File, file *ast.File, model types.Model, mode Mode) (*Info, []token.Diagnostic) {
	c := &checker{
		unit:  unit,
		model: model,
		mode:  mode,
		info: &Info{
			Types:  map[ast.Node]types.Type{},
			Aligns: map[ast.Node]int64{},
			Consts: map[ast.Expr]int64{},
			Enums:  map[*ast.Enumerator]int64{},
			Sends:  map[*ast.MessageExpr]*types.Method{},
			Props:  map[*ast.MemberExpr]*types.Property{},

			Generics:      map[*ast.GenericExpr]ast.Expr{},
			Overloads:     map[*ast.Ident]ast.Node{},
			OverloadIndex: map[ast.Node]int{},

			Captures: map[*ast.BlockLit][]Capture{},
		},
		classes:    map[string]*types.Class{},
		protocols:  map[string]*types.Protocol{},
		selectors:  map[string]struct{}{},
		undeclared: map[string]bool{},
	}
	c.push() // file scope
	c.declareBuiltinTypes()

	// Pass 1: the hierarchy.
	for _, d := range file.Decls {
		c.declareObjC(d)
	}
	// Pass 2: everything, in written order.
	for _, d := range file.Decls {
		c.checkDecl(d, true)
	}
	c.pop()

	c.info.Classes = c.classOrder
	c.info.Protocols = c.protocolOrder
	c.info.Selectors = c.selectorOrder
	token.SortDiagnostics(c.diags)
	return c.info, c.diags
}

type checker struct {
	unit  *token.File
	model types.Model
	mode  Mode
	info  *Info
	diags []token.Diagnostic

	scopes []*scope

	// The Objective-C namespaces, which are flat.
	classes       map[string]*types.Class
	protocols     map[string]*types.Protocol
	selectors     map[string]struct{}
	classOrder    []*types.Class
	protocolOrder []*types.Protocol
	selectorOrder []string

	// typeParams is the generic parameters of the class being read, if any.
	// They are erased outside it, so this is a scope of its own rather than
	// a namespace.
	typeParams map[string]*types.TypeParam

	// per-function and per-method state
	labels  map[string]*ast.LabeledStmt
	gotos   []*ast.GotoStmt
	switchD int
	loopD   int
	fnRet   types.Type

	// inferred is the return type a block's returns agree on, while fnRet
	// is nil because the block did not write one (§6.9).
	inferred types.Type

	// blocks is the block literals whose bodies are open, outermost first.
	// A name looked up while any of them is open may be a capture; see
	// noteCapture.
	blocks []*blockScope

	// self is the class whose method is being checked, and meth the method.
	// Both are nil in a function. They are what `self`, `super`, an
	// unqualified instance variable and `instancetype` all resolve through.
	self *types.Class
	meth *types.Method

	// inMethodRet is set while a method's return type is being built, which
	// is the one position §5.4 admits `instancetype` in.
	inMethodRet bool

	// inCatch counts the @catch clauses enclosing the statement being
	// checked. §7.2 makes a bare @throw valid only inside one.
	inCatch int

	// quiet suppresses reporting while an expression is typed for its type
	// alone — sizeof's operand, which is walked again where it is
	// evaluated.
	quiet int

	// undeclared remembers the names already reported, so one misspelling
	// in a loop body is one diagnostic rather than one per use.
	undeclared map[string]bool
}

func (c *checker) report(n ast.Node, msg string) {
	if n == nil || c.quiet > 0 {
		return
	}
	c.diags = append(c.diags, token.Diagnostic{
		Pos: n.Pos(), End: n.End(), Severity: token.Error, Message: msg,
	})
}

func (c *checker) warn(n ast.Node, msg string) {
	if n == nil || c.quiet > 0 {
		return
	}
	c.diags = append(c.diags, token.Diagnostic{
		Pos: n.Pos(), End: n.End(), Severity: token.Warn, Message: msg,
	})
}

// note attaches a second position to the diagnostic just reported. The
// analyzer's diagnostics are token.Diagnostics, which carry one span, so a
// note is a second diagnostic at Note severity — sorted next to the first by
// position, which is where a reader looks for it.
func (c *checker) note(n ast.Node, msg string) {
	if n == nil || c.quiet > 0 {
		return
	}
	c.diags = append(c.diags, token.Diagnostic{
		Pos: n.Pos(), End: n.End(), Severity: token.Note, Message: msg,
	})
}

func (c *checker) name(id *ast.Ident) string {
	if id == nil {
		return ""
	}
	return id.Name(c.unit)
}

func (c *checker) arc() bool { return c.mode&ARC != 0 }

// ARC is arc for the types package, which asks it through
// types.ARCResolver when it builds a parameter.
func (c *checker) ARC() bool { return c.arc() }

// ---- types.Resolver ----
//
// Construction asks the analyzer what names mean; these five methods are the
// whole of that conversation.

func (c *checker) Typedef(id *ast.Ident) types.Type {
	name := c.name(id)
	if s := c.lookup(name); s != nil && s.kind == symTypedef {
		return s.typ
	}
	if t, ok := c.builtinTypeName(name); ok {
		return t
	}
	c.report(id, "unknown type name '"+name+"'")
	return types.Typ(types.Int)
}

// builtinTypeName is a type the compiler provides and no header declares.
//
// There is one of them, and it is the reason: Apple's <sys/_types/_va_list.h>
// writes `typedef __builtin_va_list va_list;` and declares __builtin_va_list
// nowhere, because gcc and clang both supply it. A compiler that did not
// would fail on the first header that reaches <stdarg.h>, which on Darwin is
// approximately all of them.
//
// Its shape is the target's: a pointer where every variadic argument takes
// one stack slot, and an object of several words where the walk has two
// regions to cross. types.Model.VaListSize says which, and the type is
// opaque either way — a program declares one, passes it and copies it, and
// only va_start and va_arg look inside.
func (c *checker) builtinTypeName(name string) (types.Type, bool) {
	switch name {
	case "__builtin_va_list":
		if n := c.model.VaListSize; n > c.model.SizePtr {
			// An array, so that passing one decays to its address — which
			// is what makes `void f(va_list ap)` work on an ABI whose list
			// is four fields. clang says the same thing with
			// `struct __va_list_tag[1]`. The element is a long rather than
			// a char because the fields inside are pointers and the object
			// has to be aligned for them.
			return &types.Array{
				Elem: types.Typ(types.Long),
				Len:  n / c.model.SizeLong,
				Form: types.FixedArray,
			}, true
		}
		return &types.Pointer{Elem: types.Typ(types.Void)}, true

	// clang predefines these two as typedefs of __int128 and unsigned
	// __int128, and Apple's <mach/arm/_structs.h> declares the NEON
	// register file with them.
	case "__int128_t":
		return types.Typ(types.Int128), true
	case "__uint128_t":
		return types.Typ(types.UInt128), true
	}
	return nil, false
}

func (c *checker) Report(n ast.Node, msg string) { c.report(n, msg) }

func (c *checker) TypeOf(e ast.Expr) types.Type {
	t := c.quietType(e)
	if t == nil {
		return types.Typ(types.Int)
	}
	return t
}

func (c *checker) Eval(e ast.Expr) (int64, bool) { return c.evalInt(e) }

// Object resolves §5.4's ObjectTypeSpecifier.
//
// What comes back is what the specifier names, and that is not one shape: a
// class name is an interface type, because the source writes the star and
// the declarator builds the pointer, while id, Class and instancetype are
// already pointers — the language spells them without one.
func (c *checker) Object(spec *ast.ObjectType) types.Type {
	o := &types.Object{}
	var self types.Type
	switch spec.Kind {
	case ast.ObjectID:
	case ast.ObjectClass:
		o.Meta = true
	case ast.ObjectInstancetype:
		o.Instancetype = true
		if !c.inMethodRet {
			c.report(spec, "'instancetype' is valid only as the return type of a method")
		}
	case ast.ObjectTypeParam:
		name := c.name(spec.Name)
		if p, ok := c.typeParams[name]; ok {
			self = p
		} else {
			self = types.ID()
		}
	case ast.ObjectNamed:
		k := c.class(c.name(spec.Name))
		o.Base = k
		if spec.TypeArgs != nil {
			c.checkTypeArgs(spec, k)
		}
	}
	if spec.TypeArgs != nil {
		for _, a := range spec.TypeArgs.Args {
			o.Args = append(o.Args, c.typeName(a))
		}
	}
	if spec.Protocols != nil {
		for _, n := range spec.Protocols.Names {
			p := c.protocol(c.name(n))
			// Declared anywhere in the unit, which is what §4.3's forward
			// form is for — not defined above this point.
			if !p.Declared {
				c.warn(n, "protocol '"+p.Name+"' is used before it is declared")
			}
			o.Protocols = append(o.Protocols, p)
		}
	}
	if self != nil {
		// A type parameter is erased: a value of one is an object pointer,
		// and what it may hold is the bound.
		return self
	}
	switch spec.Kind {
	case ast.ObjectNamed:
		return o // the star is the declarator's
	}
	return &types.Pointer{Elem: o}
}

// checkTypeArgs reports a specialization the class cannot take.
func (c *checker) checkTypeArgs(spec *ast.ObjectType, k *types.Class) {
	n := len(spec.TypeArgs.Args)
	switch {
	case len(k.TypeParams) == 0:
		if k.Complete {
			c.report(spec.TypeArgs, "type arguments cannot be applied to '"+k.Name+
				"', which is not a generic class")
		}
	case n != len(k.TypeParams):
		c.report(spec.TypeArgs, "'"+k.Name+"' takes "+plural(len(k.TypeParams), "type argument")+
			", not "+itoa(n))
	}
}

// Tag resolves a struct, union, enum or _Atomic() specifier.
func (c *checker) Tag(spec ast.Expr) types.Type {
	switch s := spec.(type) {
	case *ast.StructType:
		return c.recordType(s)
	case *ast.EnumDecl:
		return c.enumType(s)
	case *ast.AtomicType:
		if s.Type == nil {
			return types.Typ(types.Int)
		}
		return types.Qualify(c.typeName(s.Type), types.QAtomic)
	case *ast.TypeName:
		return c.typeName(s)
	}
	return types.Typ(types.Int)
}

// typeName builds a type name and records what it denotes.
func (c *checker) typeName(tn *ast.TypeName) types.Type {
	if tn == nil {
		return nil
	}
	sp := types.BuildSpecs(c.unit, tn.Specs, c)
	t, _ := types.BuildDeclarator(c.unit, sp.Type, tn.Decl, false, c)
	c.info.Types[tn] = t
	return t
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

func plural(n int, noun string) string {
	s := itoa(n) + " " + noun
	if n != 1 {
		s += "s"
	}
	return s
}
