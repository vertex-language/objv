package analyzer

import (
	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/token"
	"github.com/vertex-language/objv/types"
)

// ---- the dispatch ----

func (c *checker) checkDecl(d ast.Decl, external bool) {
	switch d := d.(type) {
	case *ast.GenDecl:
		c.checkGenDecl(d, external)
	case *ast.FuncDecl:
		c.checkFuncDecl(d)
	case *ast.StaticAssertDecl:
		c.checkStaticAssert(d)

	// The six forms of §4, and the three of §4.4. The hierarchy is already
	// built; what happens here is the members, and the checks that need
	// them.
	case *ast.ClassInterfaceDecl:
		c.checkInterface(d)
	case *ast.CategoryDecl:
		c.checkCategory(d)
	case *ast.ProtocolDecl:
		c.checkProtocol(d)
	case *ast.ClassImplDecl:
		c.checkClassImpl(d)
	case *ast.CategoryImplDecl:
		c.checkCategoryImpl(d)
	case *ast.ClassForwardDecl, *ast.ProtocolForwardDecl, *ast.CompatAliasDecl:
		// Names, and pass 1 has them.
	case *ast.ImportDecl:
		// §4.4's module import. Resolving one needs a module map, which
		// objv does not read; the preprocessor's #import is how a header
		// arrives, and this is accepted and carries nothing.

	case *ast.AsmStmt:
		// File-scope assembly declares nothing this package can check. Its
		// text is not read here and it names no object, so there is no
		// type, no linkage and no redeclaration to reconcile.

	case *ast.EmptyDecl, *ast.BadDecl, *ast.MethodDecl, *ast.PropertyDecl,
		*ast.PropertyImplDecl, *ast.RequirementDecl, *ast.VisibilityDecl:
		// Reported by the parser, or handled by the member walk that owns
		// them.
	}
}

// ---- ordinary declarations ----

func (c *checker) checkGenDecl(d *ast.GenDecl, external bool) {
	sp := types.BuildSpecs(c.unit, d.Specs, c)
	c.checkStorage(d, sp, external)

	if len(d.List) == 0 {
		// struct S {…};  enum E {…}; — the specifier was the point. A bare
		// basic type declares nothing.
		if _, ok := types.Unqualify(sp.Type).(*types.Basic); ok {
			c.report(d, "declaration declares nothing")
		}
		return
	}

	var autoT types.Type // what __auto_type deduced, for the second declarator on

	for _, id := range d.List {
		base := sp.Type
		if sp.Auto {
			base = c.autoType(id, &autoT)
		}
		t, name := types.BuildDeclarator(c.unit, base, id.Decl, false, c)
		t = c.completeArray(t, id.Init)
		t = c.ownership(t, sp.Storage == token.EXTERN || external)
		c.info.Types[id] = t
		if name == nil {
			c.expr(id.Init)
			continue
		}
		under := types.Unqualify(t)

		if c.hasVLA(t) && (external || sp.Storage == token.STATIC || sp.Storage == token.EXTERN) {
			c.report(id, "variably modified type requires block scope and automatic storage")
		}

		sym := &symbol{typ: t, node: id, block: sp.Block,
			extern: external || sp.Storage == token.EXTERN,
			static: !external && sp.Storage == token.STATIC}
		switch {
		case sp.Storage == token.TYPEDEF:
			sym.kind = symTypedef
			if id.Init != nil {
				c.report(id, "typedef declares no object; it cannot be initialized")
			}
		case under.Kind() == types.FuncKind:
			sym.kind = symFunc
			if id.Init != nil {
				c.report(id, "function declared like a variable cannot be initialized")
			}
		default:
			sym.kind = symObject
			if sp.Inline || sp.Noreturn {
				c.report(id, "function specifiers apply only to functions")
			}
			if sp.Block && (external || sp.Storage == token.STATIC) {
				// §5.1: __block shares a local with the blocks that capture
				// it. A static or file-scope variable is already shared, so
				// there is nothing for the qualifier to arrange.
				c.report(id, "__block applies to a variable with automatic storage")
			}
			if !types.Complete(t) {
				arr, isArr := under.(*types.Array)
				deferred := sp.Storage == token.EXTERN ||
					(isArr && arr.Form == types.IncompleteArray && id.Init != nil)
				if !deferred {
					if under.Kind() == types.ObjectKind {
						// The declaration that gave the type model its
						// shape: an interface is a type, a program holds a
						// pointer to one.
						c.report(id, "interface type '"+under.String()+
							"' cannot be declared by value; declare a pointer")
					} else {
						c.report(id, "'"+c.name(name)+"' has incomplete type "+t.String())
					}
				}
			}
		}
		c.declare(name, sym)
		if sp.Auto {
			continue // autoType already walked this initializer
		}
		// An identifier's scope begins just after its declarator, so the
		// initializer is analyzed with the name already visible. That is
		// what makes `struct node *n = malloc(sizeof *n);` legal.
		init := c.expr(id.Init)
		if id.Init != nil && sym.kind == symObject && types.IsScalar(t) {
			if _, braced := id.Init.(*ast.InitList); !braced {
				c.checkAssign(id.Init, t, init, "initializing")
			}
		}
		// §6.7.9p4: an object with static storage duration is initialized
		// by a constant expression, and §6.6 says what one is. Folding it
		// here is what keeps a single evaluator in the compiler: `-1` is an
		// operator applied to a literal, and a phase that knows only
		// literals cannot initialize `static const CFIndex kCFNotFound =
		// -1;`, which <CFBase.h> writes and every Objective-C program on
		// Darwin reads. Nothing is reported when it does not fold — an
		// address constant is equally valid there and is not an integer,
		// and at block scope an initializer need not be constant at all.
		if id.Init != nil && sym.kind == symObject {
			c.foldInitializer(id.Init)
		}
	}
}

// foldInitializer records every integer constant expression an initializer
// contains, at whatever depth.
//
// §6.7.9p4: an object with static storage duration is initialized by
// constant expressions, and §6.6 says what one is. Folding them here is what
// keeps a single evaluator in the compiler — `-1` is an operator applied to
// a literal, and a phase that knows only literals cannot initialize
// `static const CFIndex kCFNotFound = -1;`, which <CFBase.h> writes and
// every Objective-C program on Darwin reads. It reaches into braces because
// a bit-field's initializer is down there and is packed into bytes rather
// than emitted as a value.
//
// Nothing is reported when an expression does not fold. An address constant
// is equally valid in the same place and is not an integer, and at block
// scope an initializer need not be constant at all — what is recorded here
// is only what *is* one, which is true wherever it was written.
func (c *checker) foldInitializer(e ast.Expr) {
	ast.Inspect(e, func(n ast.Node) bool {
		x, ok := n.(ast.Expr)
		if !ok {
			return true
		}
		if _, isList := x.(*ast.InitList); isList {
			return true
		}
		if v, ok := c.evalInt(x); ok {
			c.info.Consts[x] = v
			return false // its subexpressions are constants of this one
		}
		return true
	})
}

func (c *checker) checkStorage(d ast.Node, sp types.Spec, external bool) {
	if external && (sp.Storage == token.AUTO || sp.Storage == token.REGISTER) {
		c.report(d, "file-scope declaration cannot be auto or register")
	}
	for _, a := range sp.Aligns {
		if a.X != nil {
			if v, ok := c.requireConst(a.X, "_Alignas argument"); ok {
				if v != 0 && (v&(v-1)) != 0 {
					c.report(a, "_Alignas argument must be a power of two")
				}
			}
		}
	}
}

func (c *checker) hasVLA(t types.Type) bool {
	switch t := types.Unqualify(t).(type) {
	case *types.Array:
		return t.Form == types.VLA || c.hasVLA(t.Elem)
	case *types.Pointer:
		return c.hasVLA(t.Elem)
	}
	return false
}

// autoType deduces what __auto_type stands for in one declarator: the
// initializer's type after the conversions of C11 §6.3.2.1 and with the
// top-level qualifiers dropped.
func (c *checker) autoType(id *ast.InitDeclarator, first *types.Type) types.Type {
	if id.Init == nil {
		c.report(id, "__auto_type requires an initializer")
		return types.Typ(types.Int)
	}
	if _, braced := id.Init.(*ast.InitList); braced {
		c.report(id, "__auto_type cannot be deduced from a braced initializer")
		return types.Typ(types.Int)
	}
	if !plainNameDeclarator(id.Decl) {
		c.report(id, "__auto_type deduces the type of a plain identifier; "+
			"this declarator derives from it")
		return types.Typ(types.Int)
	}
	t := c.expr(id.Init)
	if t == nil {
		return types.Typ(types.Int)
	}
	t = types.Unqualify(types.Decay(t))
	if *first == nil {
		*first = t
		return t
	}
	if !types.Compatible(*first, t) {
		c.report(id, "__auto_type deduces "+(*first).String()+
			" earlier in this declaration and "+t.String()+" here")
		return *first
	}
	return *first
}

func plainNameDeclarator(d ast.Declarator) bool {
	for {
		switch x := d.(type) {
		case *ast.NameDeclarator:
			return true
		case *ast.ParenDeclarator:
			d = x.Inner
		default:
			return false
		}
	}
}

func (c *checker) checkStaticAssert(d *ast.StaticAssertDecl) {
	v, ok := c.requireConst(d.Cond, "_Static_assert condition")
	if ok && v == 0 {
		msg := "static assertion failed"
		if d.Msg != nil {
			sv := DecodeString(c.unit, d.Msg, c.model, func(string) {})
			b := make([]byte, 0, len(sv.Data))
			for _, u := range sv.Data[:len(sv.Data)-1] {
				b = append(b, byte(u))
			}
			msg += ": " + string(b)
		}
		c.report(d, msg)
	}
}

// ---- records and enumerations ----

func (c *checker) recordType(st *ast.StructType) types.Type {
	name := ""
	if st.Name != nil {
		name = c.name(st.Name)
	}

	var rec *types.Record
	if name != "" {
		if prev := c.lookupTag(name); prev != nil {
			r, ok := prev.typ.(*types.Record)
			switch {
			case !ok || r.Union != (st.Kind == token.UNION):
				c.report(st, "'"+name+"' declared as a different kind of tag")
			case st.Lbrace.IsValid() && r.Complete && c.currentTag(name) == prev:
				c.report(st, "redefinition of '"+prev.typ.String()+"'")
			case !st.Lbrace.IsValid() || c.currentTag(name) == prev:
				rec = r // refer to, or complete, the existing tag
			}
		}
	}
	if rec == nil {
		rec = &types.Record{Union: st.Kind == token.UNION, Name: name}
		if name != "" {
			c.declareTag(name, &tagsym{typ: rec, node: st})
		}
	}
	if !st.Lbrace.IsValid() {
		return rec
	}
	c.applyRecordAttrs(rec, st.Attrs)

	// The ceiling `#pragma pack` had in force where the specifier was
	// written. It is a property of the declaration site — <mach/message.h>
	// wraps three hundred lines in `#pragma pack(push, 4)` — and it applies
	// whether or not the struct also carries an attribute, since the two
	// answer different questions: the pragma caps every member's alignment,
	// __attribute__((packed)) removes the padding outright.
	rec.Pack = st.Pack

	if st.Defs != nil {
		// §5.8's @defs, which yields a class's instance-variable layout as
		// members. It is legacy-runtime only, and the modern runtime — the
		// only one objv emits for — decides ivar offsets when the program
		// loads, so there is no layout here to yield.
		c.report(st.Defs, "@defs is supported only under the legacy runtime")
		rec.Complete = true
		return rec
	}

	for i, f := range st.Fields {
		fd, ok := f.(*ast.FieldDecl)
		if !ok {
			c.checkDecl(f, false) // StaticAssertDecl
			continue
		}
		sp := types.BuildSpecs(c.unit, fd.Specs, c)
		if len(fd.List) == 0 {
			if anonymousMember(fd.Specs, sp.Type) {
				rec.Fields = append(rec.Fields, types.Field{Type: sp.Type})
			} else {
				c.report(fd, "declaration declares nothing")
			}
			continue
		}
		for j, d := range fd.List {
			t, id := types.BuildDeclarator(c.unit, sp.Type, d.Decl, false, c)
			fld := types.Field{Type: t}
			if id != nil {
				fld.Name = c.name(id)
			}
			if d.Colon.IsValid() {
				fld.BitField = true
				fld.Width = c.bitFieldWidth(d, t)
			} else if !types.Complete(t) {
				isFAM := false
				if a, ok := t.(*types.Array); ok && a.Form == types.IncompleteArray {
					isFAM = rec.Union ||
						i == len(st.Fields)-1 && j == len(fd.List)-1 && len(rec.Fields) > 0
				}
				if !isFAM {
					c.report(d, "member has incomplete type "+t.String())
				}
			}
			if types.IsObjCObject(t) && c.arc() {
				// An object pointer in a struct is a field ARC cannot
				// manage: there is no place to run a release, because a
				// struct has no destructor.
				c.report(d, "ARC forbids an object pointer '"+t.String()+
					"' in a struct; use __unsafe_unretained or a class")
			}
			c.info.Types[d] = t
			rec.Fields = append(rec.Fields, fld)
		}
	}
	rec.Complete = true
	return rec
}

// anonymousMember reports whether a member declaration carrying no
// declarator is an anonymous member rather than a declaration of nothing: a
// struct or union defined right there, with no tag and no member name, whose
// members belong to the record containing it.
func anonymousMember(specs ast.DeclSpecs, t types.Type) bool {
	r, ok := types.Unqualify(t).(*types.Record)
	if !ok || r.Name != "" {
		return false
	}
	for _, s := range specs {
		if _, ok := s.(*ast.StructType); ok {
			return true
		}
	}
	return false
}

func (c *checker) applyRecordAttrs(rec *types.Record, attrs []*ast.Attr) {
	for _, a := range attrs {
		switch c.attrName(a) {
		case "packed":
			rec.Packed = true
		case "aligned":
			if a.Args != nil && len(a.Args.List) > 0 {
				if v, ok := c.attrInt(a); ok {
					rec.Align = v
				}
			}
		}
	}
}

// attrName is an attribute's name with the two leading and trailing
// underscores stripped: every attribute may be written either way, so that a
// macro named `packed` cannot break a header that says `__packed__`.
func (c *checker) attrName(a *ast.Attr) string {
	if a == nil || a.Name == nil {
		return ""
	}
	n := c.name(a.Name)
	for len(n) > 4 && n[:2] == "__" && n[len(n)-2:] == "__" {
		n = n[2 : len(n)-2]
	}
	return n
}

// attrInt reads an attribute's single integer argument out of its tokens.
// §5.9 keeps them unparsed, so this is where one is read back.
func (c *checker) attrInt(a *ast.Attr) (int64, bool) {
	if a.Args == nil || len(a.Args.List) != 1 {
		return 0, false
	}
	t := a.Args.List[0]
	if t.Kind != token.INT_LIT {
		return 0, false
	}
	v := DecodeIntConst(string(c.unit.Slice(t.Pos, t.End)), c.model, func(string) {})
	return int64(v.Value), true
}

func (c *checker) bitFieldWidth(d *ast.FieldDeclarator, t types.Type) int64 {
	w, ok := c.requireConst(d.Width, "bit-field width")
	if !ok {
		return 0
	}
	if !types.IsInteger(t) {
		c.report(d, "bit-field has non-integer type "+t.String())
		return 0
	}
	bits, _ := c.model.IntBits(t)
	switch {
	case w < 0:
		c.report(d, "bit-field width is negative")
		return 0
	case w == 0 && d.Decl != nil:
		c.report(d, "a zero-width bit-field must be unnamed")
		return 0
	case w > bits:
		c.report(d, "bit-field is wider than its type "+t.String())
		return bits
	}
	return w
}

func (c *checker) enumType(ed *ast.EnumDecl) types.Type {
	name := ""
	if ed.Name != nil {
		name = c.name(ed.Name)
	}

	var en *types.Enum
	if name != "" {
		if prev := c.lookupTag(name); prev != nil {
			e, ok := prev.typ.(*types.Enum)
			switch {
			case !ok:
				c.report(ed, "'"+name+"' declared as a different kind of tag")
			case ed.Lbrace.IsValid() && e.Defined && c.currentTag(name) == prev:
				c.report(ed, "redefinition of 'enum "+name+"'")
			default:
				en = e
			}
		}
	}
	if en == nil {
		en = &types.Enum{Name: name}
		if name != "" {
			c.declareTag(name, &tagsym{typ: en, node: ed})
		}
	}
	c.info.Types[ed] = en

	// §5.8's fixed underlying type, which NS_ENUM expands to. It completes
	// the enumeration at the specifier — `enum E : NSInteger E;` is a
	// declaration of a variable of a complete type — and it decides what the
	// constants' type is.
	if ed.Base != nil {
		base := c.typeName(ed.Base)
		if !types.IsInteger(base) {
			c.report(ed.Base, "an enumeration's underlying type must be an integer type, not "+
				base.String())
		} else {
			en.Fixed = true
			en.Under = types.Unqualify(base).Kind()
			if e, ok := types.Unqualify(base).(*types.Enum); ok {
				en.Under = e.Underlying()
			}
			en.Complete = true
		}
	}

	if !ed.Lbrace.IsValid() {
		if !en.Complete {
			c.report(ed, "enum '"+name+"' is incomplete here")
		}
		return en
	}

	next := int64(0)
	values := make([]int64, 0, len(ed.List))
	syms := make([]*symbol, 0, len(ed.List))
	for _, e := range ed.List {
		if e.Value != nil {
			if v, ok := c.requireConst(e.Value, "enumerator value"); ok {
				next = v
			}
		}
		values = append(values, next)
		c.info.Enums[e] = next
		// Declared as the list is walked, because an enumerator is in scope
		// for the ones after it: `A, B = A + 1` is the whole reason the
		// value of one can be written in terms of another.
		s := &symbol{kind: symEnumConst, typ: types.Typ(types.Int), node: e, value: next}
		syms = append(syms, s)
		c.declare(e.Name, s)
		next++
	}

	if !en.Fixed {
		en.Under = c.enumUnder(values)
	} else {
		c.checkEnumFits(ed, en, values)
	}
	for _, s := range syms {
		s.typ = en.ConstType()
	}
	en.Complete = true
	en.Defined = true
	return en
}

// checkEnumFits reports an enumerator too large for the underlying type the
// program named. Without a fixed type the type is chosen to fit; with one
// there is nothing to choose.
func (c *checker) checkEnumFits(ed *ast.EnumDecl, en *types.Enum, values []int64) {
	t := types.Typ(en.Underlying())
	max := c.model.IntMax(t) // unsigned: as wide as uint64 goes

	// The comparison is unsigned where the type is, because the maximum of
	// an unsigned long does not fit in an int64: read as one it is -1, and
	// every enumerator in every CF_OPTIONS in the SDK is "too large".
	//
	// A value whose top bit is set arrives here as a negative int64 and is
	// not out of range — `NSAlignRectFlipped = 1ULL << 63` is exactly
	// representable in the unsigned long long it was declared with. The
	// conversion to uint64 is the whole check: a value too wide for a
	// narrower unsigned type is still greater than its maximum.
	if !types.IsSigned(t) {
		for i, v := range values {
			if uint64(v) > max {
				c.report(ed.List[i],
					"enumerator value does not fit in the underlying type "+t.String())
				return
			}
		}
		return
	}
	hi := int64(max)
	for i, v := range values {
		if v < -hi-1 || v > hi {
			c.report(ed.List[i],
				"enumerator value does not fit in the underlying type "+t.String())
			return
		}
	}
}

// enumUnder is the integer type an enumeration's values fit in — Invalid
// where they all fit in int, which is every enumeration C11 §6.7.2.2p2
// allows. Beyond that, the narrowest of unsigned int, long, unsigned long
// and long long that holds every value.
func (c *checker) enumUnder(values []int64) types.Kind {
	fits := func(k types.Kind) bool {
		t := types.Typ(k)
		hi := int64(c.model.IntMax(t))
		lo := int64(0)
		if types.IsSigned(t) {
			lo = -hi - 1
		}
		for _, v := range values {
			if v < lo || v > hi {
				return false
			}
		}
		return true
	}
	if fits(types.Int) {
		return types.Invalid
	}
	for _, k := range []types.Kind{types.UInt, types.Long, types.ULong, types.LongLong} {
		if fits(k) {
			return k
		}
	}
	return types.LongLong
}

// ---- functions ----

func (c *checker) checkFuncDecl(fn *ast.FuncDecl) {
	sp := types.BuildSpecs(c.unit, fn.Specs, c)
	t, name := types.BuildDeclarator(c.unit, sp.Type, fn.Decl, false, c)
	c.info.Types[fn] = t

	ft, ok := types.Unqualify(t).(*types.Func)
	if !ok {
		c.report(fn, "function definition requires a function declarator")
		return
	}
	if name != nil {
		c.declare(name, &symbol{kind: symFunc, typ: t, node: fn, extern: true})
	}

	c.push()
	c.declareFnParams(fn, ft)
	c.declareFuncName(fn, name)

	prevLabels, prevGotos, prevRet := c.labels, c.gotos, c.fnRet
	c.labels, c.gotos, c.fnRet = map[string]*ast.LabeledStmt{}, nil, ft.Ret
	c.checkStmt(fn.Body, false) // scope already pushed
	c.checkLabels()
	c.labels, c.gotos, c.fnRet = prevLabels, prevGotos, prevRet
	c.pop()
}

func (c *checker) checkLabels() {
	for _, g := range c.gotos {
		if g.Label != nil && c.labels[c.name(g.Label)] == nil {
			c.report(g, "goto to undefined label '"+c.name(g.Label)+"'")
		}
	}
}

// declareFuncName declares C11 §6.4.2.2's predefined identifier, and gcc's
// two spellings of it. It is a declaration, not a macro, which is why it
// belongs here and not in phase 4.
func (c *checker) declareFuncName(fn ast.Node, name *ast.Ident) {
	text := "<anonymous>"
	if name != nil {
		text = c.name(name)
	}
	c.declareFuncNameText(fn, text)
}

func (c *checker) declareFuncNameText(at ast.Node, text string) {
	t := types.Qualify(&types.Array{
		Elem: types.Typ(types.Char),
		Form: types.FixedArray,
		Len:  int64(len(text)) + 1,
	}, types.QConst)
	for _, spelling := range [...]string{"__func__", "__FUNCTION__", "__PRETTY_FUNCTION__"} {
		c.declareName(spelling, &symbol{kind: symObject, typ: t, node: at})
	}
}

// declareFnParams brings the definition's parameters into the body scope:
// prototype parameters directly, identifier-list parameters through the
// declaration list matched against them — the check the parser deferred.
func (c *checker) declareFnParams(fn *ast.FuncDecl, ft *types.Func) {
	fd := outermostFunc(fn.Decl)
	if fd == nil {
		return
	}

	if len(fd.Idents) == 0 && len(fn.KR) == 0 {
		for _, p := range fd.Params {
			if id := paramIdent(p); id != nil {
				var t types.Type = types.Typ(types.Int)
				for _, fp := range ft.Params {
					if fp.Name == c.name(id) {
						t = fp.Type
					}
				}
				c.declare(id, &symbol{kind: symObject, typ: t, node: p})
			}
		}
		return
	}

	named := map[string]*ast.Ident{}
	for _, id := range fd.Idents {
		named[c.name(id)] = id
	}
	declared := map[string]types.Type{}
	for _, kd := range fn.KR {
		ksp := types.BuildSpecs(c.unit, kd.Specs, c)
		if ksp.Storage != 0 && ksp.Storage != token.REGISTER {
			c.report(kd, "parameter declarations take only register")
		}
		for _, kid := range kd.List {
			t, id := types.BuildDeclarator(c.unit, ksp.Type, kid.Decl, true, c)
			if id == nil {
				continue
			}
			n := c.name(id)
			if named[n] == nil {
				c.report(id, "'"+n+"' declared but not in the parameter list")
				continue
			}
			declared[n] = types.AdjustParam(t)
		}
	}
	params := make([]types.Param, 0, len(fd.Idents))
	for _, id := range fd.Idents {
		t := declared[c.name(id)]
		if t == nil {
			t = types.Typ(types.Int)
		}
		c.declare(id, &symbol{kind: symObject, typ: t, node: id})
		params = append(params, types.Param{Name: c.name(id), Type: t})
	}
	ft.Params = params
}

func outermostFunc(d ast.Declarator) *ast.FuncDeclarator {
	var found *ast.FuncDeclarator
	for {
		switch dd := d.(type) {
		case *ast.FuncDeclarator:
			found, d = dd, dd.Inner
		case *ast.PtrDeclarator:
			d = dd.Inner
		case *ast.BlockPtrDeclarator:
			d = dd.Inner
		case *ast.ParenDeclarator:
			d = dd.Inner
		case *ast.ArrayDeclarator:
			d = dd.Inner
		default:
			return found
		}
	}
}

func paramIdent(p *ast.ParamDecl) *ast.Ident {
	if p.Decl == nil {
		return nil
	}
	return p.Decl.DeclName()
}

// completeArray fills in the length a braced or string initializer supplies:
// `int a[] = {1,2,3}` is an int[3].
func (c *checker) completeArray(t types.Type, init ast.Expr) types.Type {
	arr, ok := types.Unqualify(t).(*types.Array)
	if !ok || arr.Form != types.IncompleteArray || init == nil {
		return t
	}
	n, ok := c.inferArrayLen(init)
	if !ok {
		return t
	}
	return types.Qualify(&types.Array{Elem: arr.Elem, Form: types.FixedArray, Len: n},
		types.QualsOf(t))
}

func (c *checker) inferArrayLen(init ast.Expr) (int64, bool) {
	switch init := init.(type) {
	case *ast.InitList:
		// A designator may place an element past the end of the run, so the
		// length is the highest index reached and not the item count.
		next, max := int64(0), int64(0)
		for _, it := range init.Items {
			for _, d := range it.Designators {
				if ix, ok := d.(*ast.IndexDesignator); ok {
					if v, ok := c.evalInt(ix.Index); ok {
						next = v
					}
				}
			}
			next++
			if next > max {
				max = next
			}
		}
		return max, true
	case *ast.StringLit:
		sv := DecodeString(c.unit, init, c.model, func(string) {})
		return int64(len(sv.Data)), true
	}
	return 0, false
}

// declareBuiltinTypes declares what §2.2 says is implicitly available in
// every Objective-C translation unit.
//
// They are typedefs in <objc/objc.h> and a forward-declared class, and a
// real translation unit imports that header — but a unit that does not still
// has them, no program may redefine them, and a compiler that only knew them
// when the header was read could not check a fragment. A declaration read
// from the header replaces these, since a typedef may redeclare a typedef.
func (c *checker) declareBuiltinTypes() {
	imp := &types.Func{Ret: types.ID(), Proto: false}
	for _, d := range []struct {
		name string
		t    types.Type
	}{
		{"id", types.ID()},
		{"Class", types.ClassObject()},
		{"SEL", types.NewSelector()},
		{"IMP", &types.Pointer{Elem: imp}},
		{"BOOL", types.Typ(types.SChar)},
	} {
		c.declareName(d.name, &symbol{kind: symTypedef, typ: d.t})
	}
	// Protocol is a class, not a typedef: <objc/objc.h> declares it with
	// @class under __OBJC__.
	c.class("Protocol")
}
