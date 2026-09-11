// Package analyzer is the semantic front end: scopes and namespaces, the
// class hierarchy, method and property resolution, expression typing, ARC,
// and the constraint checks the parser deliberately deferred.
//
// It runs in two passes over one translation unit, and the reason is
// Objective-C rather than taste. A method may send a message to a class
// declared further down the file, a category may extend a class the file
// has not reached yet, and a protocol may be adopted before it is declared.
// C needs no such thing — a name must be declared before it is used — so the
// first pass exists only to make the second one possible:
//
//	pass 1  every @interface, @protocol, @class and category, so that the
//	        hierarchy is complete before any body is read
//	pass 2  everything else, in written order
//
// What the analyzer learns it records in Info, which is the seam lower reads
// through: the type of every declaration, the method every message send
// resolves to, the value of every constant expression, the ownership of every
// object it had to decide.
package analyzer

import (
	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/token"
	"github.com/vertex-language/objv/types"
)

// Mode controls optional analysis behavior.
type Mode uint

const (
	// ARC turns on automatic reference counting: ownership qualifiers are
	// inferred, the memory-management selectors are refused, and a
	// conversion between an object pointer and a non-object pointer needs
	// one of §6.5's bridge casts.
	//
	// It is a flag rather than a default because it is one in the language:
	// -fobjc-arc decides it, __has_feature(objc_arc) reports it, and a
	// translation unit compiled without it is manual reference counting all
	// the way down.
	ARC Mode = 1 << iota
)

// Info is what analysis learned about a tree.
type Info struct {
	// Types maps declaring nodes — *ast.InitDeclarator, *ast.ParamDecl,
	// *ast.FieldDeclarator, *ast.FuncDecl, *ast.TypeName — to the type they
	// declare or denote, and every expression to its type.
	Types map[ast.Node]types.Type

	// Consts maps expressions that were required to be integer constant
	// expressions, and were, to their values.
	Consts map[ast.Expr]int64

	// Enums maps every enumerator to its value, implicit or explicit.
	Enums map[*ast.Enumerator]int64

	// Sends maps each message expression to the method it resolves to, or
	// to nil where the receiver's type did not say. A nil entry is not the
	// same as no entry: it records that the send was checked and the
	// receiver was id, which is a send the runtime resolves and this
	// compiler may not.
	Sends map[*ast.MessageExpr]*types.Method

	// Props maps each property access written with dot syntax to the
	// property it names, so that lower emits the accessor send the source
	// did not write.
	Props map[*ast.MemberExpr]*types.Property

	// Generics maps each _Generic selection to the association's value that
	// its controlling expression's type selected, or to nil where no
	// association matched and there was no default — which is an error the
	// checker has already reported. The selection is a typing question and
	// the answer belongs to whoever asked it: lower emits the expression
	// recorded here and never repeats the compatibility walk, so the two
	// passes cannot disagree about which arm the program runs.
	Generics map[*ast.GenericExpr]ast.Expr

	// Captures maps each block literal to the variables its body reached
	// out of its own scopes for, in first-mention order — which is the
	// order lower lays them out in the block literal, so that the order is
	// a property of the source and not of a map iteration.
	Captures map[*ast.BlockLit][]Capture

	// Classes, Protocols and Selectors are what the unit declared and
	// mentioned, in first-seen order — the order the runtime metadata is
	// emitted in.
	Classes   []*types.Class
	Protocols []*types.Protocol
	Selectors []string
}

// Capture is one variable a block literal captured.
//
// A block captures by value: the literal holds a copy made where the literal
// was written, and the body reads that copy. `Block` marks the exception —
// a variable declared __block is shared rather than copied, and the capture
// is of a structure the runtime may move to the heap.
type Capture struct {
	Name string
	Type types.Type

	// Block is set for a variable declared __block.
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
			Consts: map[ast.Expr]int64{},
			Enums:  map[*ast.Enumerator]int64{},
			Sends:  map[*ast.MessageExpr]*types.Method{},
			Props:  map[*ast.MemberExpr]*types.Property{},

			Generics: map[*ast.GenericExpr]ast.Expr{},

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
