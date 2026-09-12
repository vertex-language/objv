package parser

import (
	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/token"
)

// ---- external declarations (§3) ----

func (p *parser) parseExternalDecl() ast.Decl {
	lo := p.pos()

	switch p.kind() {
	case token.SEMI:
		semi := p.pos()
		p.next()
		return &ast.EmptyDecl{Span: p.span(lo), Semi: semi}

	case token.STATIC_ASSERT:
		return p.parseStaticAssert()

	case token.ASM:
		return p.parseAsmStmt(lo).(*ast.AsmStmt)

	case token.HASH:
		// A directive that reached the parser. The preprocessor is above
		// this package and the scanner already reported it once per file;
		// consuming the line keeps one stray '#' from becoming a cascade.
		p.skipDirectiveLine()
		return &ast.EmptyDecl{Span: p.span(lo)}

	case token.AT_INTERFACE, token.AT_IMPLEMENTATION, token.AT_PROTOCOL,
		token.AT_CLASS, token.AT_COMPATIBILITY_ALIAS, token.AT_IMPORT:
		return p.parseObjCDecl(lo, nil)

	case token.LBRACK:
		// `[[…]]` attributes. At file scope a bracket cannot open a message
		// send — there is no expression here to send one from — so the two
		// spellings of §5.9 are unambiguous in this position and only in it.
		if p.peekTok(1).Kind == token.LBRACK {
			attrs := p.parseAttrSpecList()
			if p.at(token.AT_INTERFACE) || p.at(token.AT_PROTOCOL) ||
				p.at(token.AT_IMPLEMENTATION) {
				return p.parseObjCDecl(lo, attrs)
			}
			return p.parseDeclOrFunc(lo, attrs)
		}
	}

	// §4.1 and §4.3 allow an attribute list before @interface and
	// @protocol, and it is where every NS_CLASS_AVAILABLE lands.
	if p.at(token.ATTRIBUTE) {
		attrs := p.parseAttrSpecList()
		switch p.kind() {
		case token.AT_INTERFACE, token.AT_PROTOCOL, token.AT_IMPLEMENTATION:
			return p.parseObjCDecl(lo, attrs)
		}
		return p.parseDeclOrFunc(lo, attrs)
	}
	return p.parseDeclOrFunc(lo, nil)
}

// parseDeclOrFunc reads a declaration, and turns it into a function
// definition when a body follows the first declarator.
func (p *parser) parseDeclOrFunc(lo token.Pos, attrs []*ast.Attr) ast.Decl {
	specs := p.parseDeclSpecs(false)
	if len(attrs) > 0 {
		specs = append(ast.DeclSpecs{&ast.AttrSpec{
			Span: ast.Span{Lo: attrs[0].Pos(), Hi: attrs[len(attrs)-1].End()}, Attrs: attrs,
		}}, specs...)
	}
	// Handle export macros preceding an @interface (e.g. OBJC_EXPORT @interface ...).
	switch p.kind() {
	case token.AT_INTERFACE, token.AT_PROTOCOL, token.AT_IMPLEMENTATION:
		return p.parseObjCDecl(lo, append(attrs, attrsOfSpecs(specs)...))
	}

	if p.at(token.SEMI) {
		semi := p.pos()
		p.next()
		return &ast.GenDecl{Span: p.span(lo), Specs: specs, Semi: semi}
	}

	isTypedef := hasKeyword(specs, token.TYPEDEF)
	d := p.parseDeclarator(dmodeNormal)
	p.declareDeclarator(d, isTypedef)

	// A body — or a K&R declaration list before one — makes it a definition.
	if p.at(token.LBRACE) || (!isTypedef && p.startsKRList()) {
		return p.parseFuncDef(lo, specs, d)
	}
	return p.finishGenDecl(lo, specs, d)
}

// attrsOfSpecs extracts attributes from a declaration specifier list.
func attrsOfSpecs(specs ast.DeclSpecs) []*ast.Attr {
	var out []*ast.Attr
	for _, s := range specs {
		if a, ok := s.(*ast.AttrSpec); ok {
			out = append(out, a.Attrs...)
		}
	}
	return out
}

// startsKRList reports whether a K&R parameter declaration list follows a
// function declarator. The form is obsolescent (§3) and is here because
// system headers still carry it.
func (p *parser) startsKRList() bool {
	if !p.isDeclSpecStart(p.tok()) {
		return false
	}
	// Only after a declarator that named its parameters as bare identifiers.
	return true
}

func (p *parser) parseFuncDef(lo token.Pos, specs ast.DeclSpecs, d ast.Declarator) ast.Decl {
	fn := &ast.FuncDecl{
		Specs: specs, Decl: d, Name: d.DeclName(),
		AsmLabel: p.takeAsmLabel(), Attrs: p.takeDeclAttrs(),
	}
	p.pushScope()
	p.declareParams(d)
	for p.startsKRList() && !p.at(token.LBRACE) && !p.at(token.EOF) {
		start := p.i
		if g, ok := p.parseDeclaration().(*ast.GenDecl); ok {
			fn.KR = append(fn.KR, g)
		}
		if p.i == start {
			break
		}
	}
	if p.mode&SkipBodies != 0 {
		fn.Body = p.skipBody()
	} else {
		fn.Body = p.parseCompound(false)
	}
	p.popScope()
	fn.Span = p.span(lo)
	return fn
}

// skipBody consumes a body balanced instead of parsing it, for SkipBodies.
func (p *parser) skipBody() *ast.CompoundStmt {
	lo := p.pos()
	cs := &ast.CompoundStmt{Lbrace: p.pos()}
	p.skipBalanced()
	cs.Rbrace = p.prevEnd() - 1
	cs.Span = p.span(lo)
	return cs
}

// parseDeclaration reads one ordinary declaration, terminator included.
func (p *parser) parseDeclaration() ast.Decl {
	lo := p.pos()
	if p.at(token.STATIC_ASSERT) {
		return p.parseStaticAssert()
	}
	specs := p.parseDeclSpecs(false)
	if p.at(token.SEMI) {
		semi := p.pos()
		p.next()
		return &ast.GenDecl{Span: p.span(lo), Specs: specs, Semi: semi}
	}
	isTypedef := hasKeyword(specs, token.TYPEDEF)
	d := p.parseDeclarator(dmodeNormal)
	p.declareDeclarator(d, isTypedef)
	return p.finishGenDecl(lo, specs, d)
}

func (p *parser) finishGenDecl(lo token.Pos, specs ast.DeclSpecs, first ast.Declarator) *ast.GenDecl {
	g := &ast.GenDecl{Specs: specs}
	isTypedef := hasKeyword(specs, token.TYPEDEF)
	d := first
	for {
		ilo := d.Pos()
		it := &ast.InitDeclarator{Decl: d, AsmLabel: p.takeAsmLabel(), Attrs: p.takeDeclAttrs()}
		if p.at(token.ASSIGN) {
			it.Assign = p.pos()
			p.next()
			it.Init = p.parseInitializer()
		}
		it.Span = ast.Span{Lo: ilo, Hi: p.widen(ilo, p.prevEnd())}
		g.List = append(g.List, it)
		if !p.at(token.COMMA) {
			break
		}
		p.next()
		start := p.i
		d = p.parseDeclarator(dmodeNormal)
		p.declareDeclarator(d, isTypedef)
		if p.i == start {
			break
		}
	}
	g.Semi = p.expectSemi()
	g.Span = p.span(lo)
	return g
}

func (p *parser) parseStaticAssert() *ast.StaticAssertDecl {
	lo := p.pos()
	s := &ast.StaticAssertDecl{Keyword: p.pos()}
	p.next()
	s.Lparen = p.expect(token.LPAREN)
	s.Cond = p.parseCond()
	if p.at(token.COMMA) {
		s.Comma = p.pos()
		p.next()
		if p.at(token.STRING_LIT) {
			s.Msg = p.parseStringRun()
		} else {
			p.errHere("expected a string literal")
		}
	}
	s.Rparen = p.expect(token.RPAREN)
	s.Semi = p.expectSemi()
	s.Span = p.span(lo)
	return s
}

// skipDirectiveLine consumes a stray '#' line.
var debugPack = false

func (p *parser) skipDirectiveLine() {
	start := p.i
	p.next()
	for !p.at(token.EOF) && !p.tok().Flags.Has(token.FlagNLBefore) {
		p.next()
	}
	p.readPragma(p.toks[start:p.i])
	if debugPack {
		println("pragma line len", p.i-start, "pack now", p.pack)
	}
}

// readPragma handles #pragma pack directives: pack(N), pack(), pack(push[, N]), pack(pop).
func (p *parser) readPragma(line []token.Token) {
	// line is `#` `pragma` `pack` `(` ... `)`.
	if len(line) < 3 || !p.tokIs(line[1], "pragma") || !p.tokIs(line[2], "pack") {
		return
	}
	args := line[3:]
	if len(args) < 2 || args[0].Kind != token.LPAREN {
		return
	}
	args = args[1:]
	if n := len(args); n > 0 && args[n-1].Kind == token.RPAREN {
		args = args[:n-1]
	}

	switch {
	case len(args) == 0:
		p.pack = 0
	case p.tokIs(args[0], "pop"):
		if n := len(p.packStack); n > 0 {
			p.pack = p.packStack[n-1]
			p.packStack = p.packStack[:n-1]
		} else {
			p.pack = 0
		}
	case p.tokIs(args[0], "push"):
		p.packStack = append(p.packStack, p.pack)
		if len(args) >= 3 && args[1].Kind == token.COMMA {
			p.pack = p.packValue(args[2])
		}
	default:
		p.pack = p.packValue(args[0])
	}
}

// packValue reads the alignment a pack pragma named. A value that is not a
// power of two is ignored rather than reported: the pragma is an extension
// with no standard to violate, and a compiler that refused one would refuse
// a header it otherwise understands.
func (p *parser) packValue(t token.Token) int64 {
	if t.Kind != token.INT_LIT {
		return p.pack
	}
	var n int64
	for _, c := range p.f.Slice(t.Pos, t.End) {
		if c < '0' || c > '9' {
			return p.pack
		}
		n = n*10 + int64(c-'0')
		if n > 1<<20 {
			return p.pack
		}
	}
	if n == 0 || n&(n-1) != 0 {
		return p.pack
	}
	return n
}

func (p *parser) tokIs(t token.Token, word string) bool {
	return string(p.f.Slice(t.Pos, t.End)) == word
}

// ---- declaration specifiers (§5) ----

// parseDeclSpecs reads a specifier list in written order. sq restricts it to
// §5.7's SpecifierQualifierList — no storage classes and no function
// specifiers — which is what a type name, a struct member and a method type
// take.
func (p *parser) parseDeclSpecs(sq bool) ast.DeclSpecs {
	var specs ast.DeclSpecs
	sawType := false
	for {
		switch p.kind() {
		case token.TYPEDEF, token.EXTERN, token.STATIC, token.THREAD_LOCAL,
			token.AUTO, token.REGISTER, token.INLINE, token.NORETURN, token.BLOCK:
			if sq {
				return specs
			}
			specs = append(specs, p.keywordSpec())
			continue

		case token.CONST, token.RESTRICT, token.VOLATILE,
			token.KINDOF, token.STRONG, token.WEAK, token.UNSAFE_UNRETAINED,
			token.AUTORELEASING, token.NONNULL, token.NULLABLE,
			token.NULL_UNSPECIFIED:
			specs = append(specs, p.keywordSpec())
			continue

		case token.VOID, token.CHAR, token.SHORT, token.INT, token.LONG,
			token.FLOAT, token.DOUBLE, token.SIGNED, token.UNSIGNED,
			token.BOOL, token.COMPLEX, token.IMAGINARY, token.AUTO_TYPE,
			token.INT128, token.FLOAT16:
			sawType = true
			specs = append(specs, p.keywordSpec())
			continue

		case token.ATOMIC:
			// _Atomic(T) is a specifier; a bare _Atomic is a qualifier.
			if p.peekTok(1).Kind == token.LPAREN {
				sawType = true
				specs = append(specs, p.parseAtomicType())
			} else {
				specs = append(specs, p.keywordSpec())
			}
			continue

		case token.ALIGNAS:
			if sq {
				return specs
			}
			specs = append(specs, p.parseAlignas())
			continue

		case token.PTRAUTH:
			specs = append(specs, p.parsePtrauth())
			continue

		case token.TYPEOF:
			sawType = true
			specs = append(specs, p.parseTypeof())
			continue

		case token.ATTRIBUTE:
			specs = append(specs, p.attrSpec(p.parseAttrSpecList()))
			continue

		case token.STRUCT, token.UNION:
			sawType = true
			specs = append(specs, p.parseStructType())
			continue

		case token.ENUM:
			sawType = true
			specs = append(specs, p.parseEnumDecl())
			continue

		case token.IDENT:
			name := p.text()
			// §5.6's underscore-free nullability spellings, which mean the
			// qualifier only inside a MethodType. They come first: they are
			// identifiers followed by a type, which is exactly the shape the
			// unknown-type recovery below looks for.
			if p.inMethodType && !sawType {
				if k, ok := nullabilityWord(name); ok {
					t := p.tok()
					p.next()
					specs = append(specs, &ast.KeywordSpec{
						Span: ast.Span{Lo: t.Pos, Hi: t.End}, Kind: k})
					continue
				}
			}
			// An identifier that is not a known type, in a position where
			// only a type may stand, is a class whose declaration has not
			// been read — `NSArray<K> *keys;` with no @class for NSArray.
			// Saying so is worth far more than the cascade that follows
			// from reading it as the declarator: the rest of the
			// declaration parses, and one diagnostic names the cause.
			if !sawType && !p.isTypeName(name) && p.startsUnknownType() {
				p.errHere("unknown type name '" + name + "'")
				p.declareGlobal(name, nameClass)
				sawType = true
				specs = append(specs, p.parseNamedType())
				continue
			}
			// A second identifier is the declarator, not another specifier:
			// `NSString name` declares name, and `id` is a type here only
			// because nothing else has claimed the slot.
			if sawType || !p.isTypeName(name) {
				return specs
			}
			sawType = true
			specs = append(specs, p.parseNamedType())
			continue
		}
		return specs
	}
}

// startsUnknownType reports whether what follows the identifier under the
// cursor can only follow a type: another identifier, a pointer or block
// pointer, or an angle-bracket list.
func (p *parser) startsUnknownType() bool {
	switch p.peekTok(1).Kind {
	case token.IDENT, token.MUL, token.XOR, token.LSS:
		return true
	}
	return false
}

func (p *parser) attrSpec(attrs []*ast.Attr) *ast.AttrSpec {
	s := &ast.AttrSpec{Attrs: attrs}
	if len(attrs) > 0 {
		s.Span = ast.Span{Lo: attrs[0].Pos(), Hi: attrs[len(attrs)-1].End()}
	}
	return s
}

func nullabilityWord(name string) (token.Kind, bool) {
	switch name {
	case "nonnull":
		return token.NONNULL, true
	case "nullable":
		return token.NULLABLE, true
	case "null_unspecified":
		return token.NULL_UNSPECIFIED, true
	}
	return token.ILLEGAL, false
}

func (p *parser) keywordSpec() *ast.KeywordSpec {
	t := p.tok()
	p.next()
	return &ast.KeywordSpec{Span: ast.Span{Lo: t.Pos, Hi: t.End}, Kind: t.Kind}
}

func (p *parser) parseAtomicType() *ast.AtomicType {
	lo := p.pos()
	a := &ast.AtomicType{Atomic: p.pos()}
	p.next()
	a.Lparen = p.expect(token.LPAREN)
	a.Type = p.parseTypeName()
	a.Rparen = p.expect(token.RPAREN)
	a.Span = p.span(lo)
	return a
}

func (p *parser) parseAlignas() *ast.AlignasSpec {
	lo := p.pos()
	a := &ast.AlignasSpec{Alignas: p.pos()}
	p.next()
	a.Lparen = p.expect(token.LPAREN)
	if p.isTypeSpecStart(p.tok()) {
		a.Type = p.parseTypeName()
	} else {
		a.X = p.parseCond()
	}
	a.Rparen = p.expect(token.RPAREN)
	a.Span = p.span(lo)
	return a
}

// parseTypeof reads §5.3's typeof. Its two alternatives are told apart by
// resolving the contents of the parentheses, with a type name preferred where
// both parse.
func (p *parser) parseTypeof() ast.Expr {
	lo := p.pos()
	t := &ast.TypeofType{Keyword: p.pos()}
	p.next()
	t.Lparen = p.expect(token.LPAREN)
	if p.isTypeSpecStart(p.tok()) {
		t.Type = p.parseTypeName()
	} else {
		t.X = p.parseExpr()
	}
	t.Rparen = p.expect(token.RPAREN)
	t.Span = p.span(lo)
	return t
}

// parsePtrauth reads §5.6's __ptrauth ( BalancedTokenSequence ).
func (p *parser) parsePtrauth() ast.Expr {
	lo := p.pos()
	s := &ast.PtrauthSpec{Keyword: p.pos()}
	p.next()
	s.Lparen = p.expect(token.LPAREN)
	s.Args = p.parseBalanced()
	s.Rparen = p.expect(token.RPAREN)
	s.Span = p.span(lo)
	return s
}

// parseNamedType reads an identifier used as a type: §5.4's
// ObjectTypeSpecifier, or an ordinary typedef name.
//
// Which it is comes from the name table, and the answer decides what may
// follow: only an object type takes the two angle-bracket lists.
func (p *parser) parseNamedType() ast.Expr {
	lo := p.pos()
	name := p.text()
	kind, object := objectKindOf(name, p.lookup(name))
	id := p.ident()
	if !object {
		return &ast.TypedefType{Span: p.span(lo), Name: id}
	}

	o := &ast.ObjectType{Kind: kind, Name: id}
	switch kind {
	case ast.ObjectID, ast.ObjectClass, ast.ObjectInstancetype:
		o.Name = nil // the spelling is the kind; keeping it would say it twice
	}
	// `NSArray<NSString *> <NSCopying>`: §4.1 admits both lists, in that
	// order, and each is told from the other by resolving the names in it.
	if p.at(token.LSS) {
		if kind == ast.ObjectID || kind == ast.ObjectClass || p.angleIsProtocolList() {
			o.Protocols = p.parseProtocolRefList()
		} else {
			o.TypeArgs = p.parseTypeArgList()
			if p.at(token.LSS) {
				o.Protocols = p.parseProtocolRefList()
			}
		}
	}
	o.Span = p.span(lo)
	return o
}

func objectKindOf(name string, k nameKind) (ast.ObjectKind, bool) {
	switch name {
	case "id":
		return ast.ObjectID, true
	case "Class":
		return ast.ObjectClass, true
	case "instancetype":
		return ast.ObjectInstancetype, true
	}
	switch k {
	case nameClass:
		return ast.ObjectNamed, true
	case nameTypeParam:
		return ast.ObjectTypeParam, true
	}
	return 0, false
}

// ---- angle-bracket lists (§4.1, §5.5) ----

// angleIsProtocolList decides which list a '<' opens after a type or a
// superclass. §4.1's rule is to resolve each identifier in it: a protocol
// name yields a ProtocolReferenceList, anything else a TypeArgumentList.
//
// The first element settles it, and there are three cases rather than two,
// because a name may be neither yet: a protocol used before it is declared
// is still a protocol, and `<NSCopying>` after a superclass is a conformance
// whether or not the header that declares it has been read. So an unknown
// name falls back to shape — a protocol list is bare names and commas, and
// nothing else is.
func (p *parser) angleIsProtocolList() bool {
	// Shape first, names second. A protocol reference list is bare
	// identifiers and nothing else, so anything with a '*' or a nested '<'
	// in it is a type-argument list whatever the identifiers are called —
	// and a name may be both. `NSArray<NSMenuItem *>` is a specialization
	// even where NSMenuItem is also a protocol, because `<NSMenuItem *>` is
	// not a list a protocol reference could be written as.
	if !p.angleHoldsOnlyNames() {
		return false
	}
	switch name := p.name(p.peekTok(1)); {
	case p.isProtocolName(name):
		return true
	case p.isTypeName(name):
		return false
	}
	// An undeclared name in a bare list: a protocol reference to something
	// declared later is far likelier than a specialization on a type that
	// does not exist.
	return true
}

// angleHoldsOnlyNames reports whether the list starting at the cursor's '<'
// is bare identifiers separated by commas — the only shape a protocol
// reference list has.
func (p *parser) angleHoldsOnlyNames() bool {
	for n := 1; n < 64; n++ {
		switch t := p.peekTok(n); t.Kind {
		case token.IDENT:
			if n%2 == 0 {
				return false // two names in a row
			}
		case token.COMMA:
			if n%2 == 1 {
				return false
			}
		case token.GTR, token.SHR, token.SHR_ASSIGN, token.GEQ:
			return n%2 == 0 // closed right after a name
		default:
			return false
		}
	}
	return false
}

// angleIsTypeParamList decides the other side of the same ambiguity: the '<'
// immediately after a class name in an @interface or an @class, where §4.1
// admits a TypeParameterList and a ProtocolReferenceList both.
//
// A variance keyword or a bound settles it outright — no protocol list has
// either. Otherwise a list of names that are all declared protocols is a
// conformance, `@interface Foo <NSCopying>`, and anything else is a
// parameter list, `@interface Container<T>`. That is clang's resolution too,
// and it is why a protocol must be declared before the class that conforms
// to it.
func (p *parser) angleIsTypeParamList() bool {
	for n := 1; n < 64; n++ {
		switch t := p.peekTok(n); t.Kind {
		case token.COVARIANT, token.CONTRAVARIANT, token.COLON:
			return true
		case token.IDENT:
			if !p.isProtocolName(p.name(t)) {
				return true
			}
		case token.COMMA:
		case token.GTR, token.SHR, token.SHR_ASSIGN, token.GEQ:
			return false // every name was a declared protocol
		default:
			return true
		}
	}
	return true
}

func (p *parser) parseProtocolRefList() *ast.ProtocolRefList {
	lo := p.pos()
	l := &ast.ProtocolRefList{Langle: p.pos()}
	p.next()
	for !p.atRangle() && !p.at(token.EOF) {
		id := p.expectIdent()
		if id == nil {
			p.advanceTo(declFollow)
			break
		}
		l.Names = append(l.Names, id)
		if !p.at(token.COMMA) {
			break
		}
		p.next()
	}
	l.Rangle = p.expectRangle()
	l.Span = p.span(lo)
	return l
}

func (p *parser) parseTypeArgList() *ast.TypeArgList {
	lo := p.pos()
	l := &ast.TypeArgList{Langle: p.pos()}
	p.next()
	for !p.atRangle() && !p.at(token.EOF) {
		start := p.i
		l.Args = append(l.Args, p.parseTypeName())
		if p.i == start {
			p.advanceTo(declFollow)
			break
		}
		if !p.at(token.COMMA) {
			break
		}
		p.next()
	}
	l.Rangle = p.expectRangle()
	l.Span = p.span(lo)
	return l
}

// parseTypeParamList reads §5.5's `< TypeParameter {, TypeParameter} >` and
// declares each parameter in the current scope, so that a bound or a member
// written in terms of one reads as a type.
func (p *parser) parseTypeParamList() *ast.TypeParamList {
	lo := p.pos()
	l := &ast.TypeParamList{Langle: p.pos()}
	p.next()
	for !p.atRangle() && !p.at(token.EOF) {
		plo := p.pos()
		tp := &ast.TypeParam{}
		if p.at(token.COVARIANT) || p.at(token.CONTRAVARIANT) {
			tp.Variance, tp.VarianceKey = p.pos(), p.kind()
			p.next()
		}
		tp.Name = p.expectIdent()
		if tp.Name == nil {
			p.advanceTo(declFollow)
			break
		}
		p.declare(tp.Name.Name(p.f), nameTypeParam)
		if p.at(token.COLON) {
			tp.Colon = p.pos()
			p.next()
			tp.Bound = p.parseTypeName()
		}
		tp.Span = p.span(plo)
		l.Params = append(l.Params, tp)
		if !p.at(token.COMMA) {
			break
		}
		p.next()
	}
	l.Rangle = p.expectRangle()
	l.Span = p.span(lo)
	return l
}

// atRangle reports whether the cursor is on something that can close an
// angle-bracket list.
func (p *parser) atRangle() bool {
	switch p.kind() {
	case token.GTR, token.SHR, token.SHR_ASSIGN, token.GEQ:
		return true
	}
	return false
}

// expectRangle consumes the '>' closing a list, splitting the token when the
// scanner munched it together with what follows.
//
// `NSArray<id<NSCopying>>` closes two lists with one SHR, because nothing
// below the parser knows a list is open (§6.6). The parser knows: it takes
// the leading '>' and writes the remainder back into the token stream for the
// outer list to close with, so both halves keep exact spans and a diagnostic
// still underlines the right character.
func (p *parser) expectRangle() token.Pos {
	switch p.kind() {
	case token.GTR:
		pos := p.pos()
		p.next()
		return pos
	case token.SHR, token.SHR_ASSIGN, token.GEQ:
		lead, rest, _ := p.tok().SplitAngle()
		p.toks[p.i] = rest
		p.quiet = false
		return lead.Pos
	}
	p.errHere("expected '>'")
	return token.NoPos
}

// ---- struct, union, enum (§5.8) ----

func (p *parser) parseStructType() *ast.StructType {
	lo := p.pos()
	s := &ast.StructType{Keyword: p.pos(), Kind: p.kind(), Pack: p.pack}
	p.next()
	if p.at(token.ATTRIBUTE) {
		s.Attrs = append(s.Attrs, p.parseAttrSpecList()...)
	}
	if p.at(token.IDENT) {
		s.Name = p.ident()
	}
	if !p.at(token.LBRACE) {
		if s.Name == nil {
			p.errHere("expected a tag or '{' after struct or union")
		}
		s.Span = p.span(lo)
		return s
	}
	s.Lbrace = p.pos()
	p.next()

	// §5.8's third alternative: the instance-variable layout of a class as
	// structure members. It is legacy-runtime only and rejected later; the
	// parse exists so the diagnostic can name what it is.
	if p.at(token.AT_DEFS) {
		dlo := p.pos()
		d := &ast.DefsSpec{Keyword: p.pos()}
		p.next()
		d.Lparen = p.expect(token.LPAREN)
		d.Name = p.expectIdent()
		d.Rparen = p.expect(token.RPAREN)
		d.Span = p.span(dlo)
		s.Defs = d
	} else {
		for !p.at(token.RBRACE) && !p.at(token.EOF) {
			start := p.i
			s.Fields = append(s.Fields, p.parseStructDeclaration())
			if p.i == start {
				p.advanceTo(declFollow)
				if p.at(token.SEMI) {
					p.next()
				}
				if p.i == start {
					p.next()
				}
			}
		}
	}
	s.Rbrace = p.expect(token.RBRACE)
	if p.at(token.ATTRIBUTE) {
		s.Attrs = append(s.Attrs, p.parseAttrSpecList()...)
	}
	s.Span = p.span(lo)
	return s
}

// parseStructDeclaration reads one member: a specifier-qualifier list and
// declarators, some of which may be bit-fields. It is also §4.5's instance
// variable declaration, which is the same production.
func (p *parser) parseStructDeclaration() ast.Decl {
	lo := p.pos()
	if p.at(token.STATIC_ASSERT) {
		return p.parseStaticAssert()
	}
	if p.at(token.SEMI) {
		semi := p.pos()
		p.next()
		return &ast.EmptyDecl{Span: p.span(lo), Semi: semi}
	}
	f := &ast.FieldDecl{Specs: p.parseDeclSpecs(true)}
	if !p.at(token.SEMI) {
		for {
			dlo := p.pos()
			fd := &ast.FieldDeclarator{}
			if !p.at(token.COLON) {
				fd.Decl = p.parseDeclarator(dmodeNormal)
			}
			if p.at(token.COLON) {
				fd.Colon = p.pos()
				p.next()
				fd.Width = p.parseCond()
			}
			p.takeAsmLabel()
			p.takeDeclAttrs()
			fd.Span = p.span(dlo)
			f.List = append(f.List, fd)
			if !p.at(token.COMMA) {
				break
			}
			p.next()
		}
	}
	f.Semi = p.expectSemi()
	f.Span = p.span(lo)
	return f
}

// parseEnumDecl reads §5.8's enum specifier, fixed underlying type included.
// That colon is what NS_ENUM expands to and what every enumeration in the
// Cocoa headers carries.
func (p *parser) parseEnumDecl() *ast.EnumDecl {
	lo := p.pos()
	e := &ast.EnumDecl{Enum: p.pos()}
	p.next()
	if p.at(token.ATTRIBUTE) {
		e.Attrs = append(e.Attrs, p.parseAttrSpecList()...)
	}
	if p.at(token.IDENT) {
		e.Name = p.ident()
	}
	if p.at(token.COLON) {
		e.Colon = p.pos()
		p.next()
		e.Base = p.parseTypeName()
	}
	if !p.at(token.LBRACE) {
		if e.Name == nil {
			p.errHere("expected a tag or '{' after enum")
		}
		e.Span = p.span(lo)
		return e
	}
	e.Lbrace = p.pos()
	p.next()
	for !p.at(token.RBRACE) && !p.at(token.EOF) {
		nlo := p.pos()
		en := &ast.Enumerator{Name: p.expectName()}
		if en.Name == nil {
			p.advanceTo(declFollow)
			break
		}
		p.declare(en.Name.Name(p.f), nameOrdinary)
		if p.at(token.ATTRIBUTE) {
			en.Attrs = p.parseAttrSpecList()
		}
		if p.at(token.ASSIGN) {
			en.Assign = p.pos()
			p.next()
			en.Value = p.parseCond()
		}
		en.Span = p.span(nlo)
		e.List = append(e.List, en)
		if !p.at(token.COMMA) {
			break
		}
		comma := p.pos()
		p.next()
		if p.at(token.RBRACE) {
			e.Comma = comma
		}
	}
	e.Rbrace = p.expect(token.RBRACE)
	if p.at(token.ATTRIBUTE) {
		e.Attrs = append(e.Attrs, p.parseAttrSpecList()...)
	}
	e.Span = p.span(lo)
	return e
}

// ---- attributes (§5.9) ----

// parseAttrSpecList reads a run of attribute specifiers in either spelling.
func (p *parser) parseAttrSpecList() []*ast.Attr {
	var out []*ast.Attr
	for {
		switch {
		case p.at(token.ATTRIBUTE):
			p.next()
			lp1 := p.expect(token.LPAREN)
			lp2 := p.expect(token.LPAREN)
			if !lp1.IsValid() || !lp2.IsValid() {
				p.advanceTo(declFollow)
				return out
			}
			out = append(out, p.parseAttrList(token.RPAREN)...)
			p.expect(token.RPAREN)
			p.expect(token.RPAREN)

		case p.at(token.LBRACK) && p.peekTok(1).Kind == token.LBRACK:
			p.next()
			p.next()
			out = append(out, p.parseAttrList(token.RBRACK)...)
			p.expect(token.RBRACK)
			p.expect(token.RBRACK)

		default:
			return out
		}
	}
}

// parseAttrList reads the comma-separated attributes inside one specifier.
// An empty list is accepted: `__attribute__(())` is well-formed.
func (p *parser) parseAttrList(closer token.Kind) []*ast.Attr {
	var out []*ast.Attr
	for !p.at(closer) && !p.at(token.EOF) {
		a := p.parseAttr()
		if a == nil {
			p.advanceTo(map[token.Kind]bool{closer: true, token.SEMI: true})
			break
		}
		out = append(out, a)
		if !p.at(token.COMMA) {
			break
		}
		p.next()
	}
	return out
}

// parseAttr reads one attribute: a name, optionally scoped, and optionally a
// parenthesized token sequence.
//
// The arguments are tokens because §5.9 says BalancedTokenSequence and means
// it — `availability(macosx, introduced=10.12.1)` parses as no expression,
// and it is on nearly every declaration in the SDK.
func (p *parser) parseAttr() *ast.Attr {
	lo := p.pos()
	if !isName(p.tok()) {
		p.errHere("expected an attribute name")
		return nil
	}
	a := &ast.Attr{Name: p.ident()}
	if p.at(token.COLON) && p.peekTok(1).Kind == token.COLON &&
		p.peekTok(1).Flags.Has(token.FlagAdjacent) {
		// A scoped name, `clang::objc_arc`. The scanner leaves '::' as two
		// adjacent colons, because a munched pair would break the selector
		// `a::` — so the adjacency flag is what says this is one token's
		// worth of punctuation.
		a.Scope, a.Colons = a.Name, p.pos()
		p.next()
		p.next()
		a.Name = p.expectName()
	}
	if p.at(token.LPAREN) {
		a.Lparen = p.pos()
		p.next()
		a.Args = p.parseBalanced()
		a.Rparen = p.expect(token.RPAREN)
	}
	a.Span = p.span(lo)
	return a
}

// parseBalanced collects §8's BalancedTokenSequence up to the closer that
// matches the opener already consumed.
func (p *parser) parseBalanced() *ast.Tokens {
	lo := p.pos()
	t := &ast.Tokens{}
	depth := 0
	for {
		switch p.kind() {
		case token.EOF:
			t.Span = p.span(lo)
			return t
		case token.LPAREN, token.LBRACK, token.LBRACE:
			depth++
		case token.RPAREN, token.RBRACK, token.RBRACE:
			if depth == 0 {
				t.Span = p.span(lo)
				return t
			}
			depth--
		}
		t.List = append(t.List, p.tok())
		p.next()
	}
}

// ---- declarators (§5.7) ----

type dmode uint8

const (
	dmodeNormal   dmode = iota // a name is expected
	dmodeAbstract              // no name (a type name)
	dmodeEither                // a parameter: either
)

func (p *parser) parseDeclarator(m dmode) ast.Declarator {
	p.depth++
	defer func() { p.depth-- }()
	lo := p.pos()
	if p.tooDeep() {
		return &ast.BadDeclarator{Span: p.span(lo)}
	}

	// An attribute may lead a declarator, where it belongs to the pointer
	// or block pointer that follows it:
	//
	//	void (__attribute__((noescape)) ^)(NSUInteger idx, BOOL *stop)
	//
	// which is how every enumerate…UsingBlock: in Foundation is declared.
	// Written after the '^' it would be an ordinary qualifier-position
	// attribute; written before, it is still one, and this is what makes
	// the two spellings mean the same thing.
	var lead ast.DeclSpecs
	if p.at(token.ATTRIBUTE) {
		attrs := p.parseAttrSpecList()
		if len(attrs) > 0 {
			lead = ast.DeclSpecs{&ast.AttrSpec{
				Span:  ast.Span{Lo: attrs[0].Pos(), Hi: attrs[len(attrs)-1].End()},
				Attrs: attrs,
			}}
		}
	}

	// Pointer: * or ^, each with its own qualifier list, and each nesting.
	switch p.kind() {
	case token.MUL:
		d := &ast.PtrDeclarator{Star: p.pos()}
		p.next()
		d.Quals = append(lead, p.parseQualList()...)
		if p.startsDeclaratorTail(m) {
			d.Inner = p.parseDeclarator(m)
		}
		d.Span = p.span(lo)
		return d

	case token.XOR:
		// §5.7's block pointer. It occupies the position a '*' would.
		d := &ast.BlockPtrDeclarator{Caret: p.pos()}
		p.next()
		d.Quals = append(lead, p.parseQualList()...)
		if p.startsDeclaratorTail(m) {
			d.Inner = p.parseDeclarator(m)
		}
		d.Span = p.span(lo)
		return d
	}
	return p.parseDirectDeclarator(m, lo)
}

// parseQualList reads the qualifiers that may follow a pointer.
func (p *parser) parseQualList() ast.DeclSpecs {
	var out ast.DeclSpecs
	for {
		switch p.kind() {
		case token.CONST, token.RESTRICT, token.VOLATILE, token.ATOMIC,
			token.KINDOF, token.STRONG, token.WEAK, token.UNSAFE_UNRETAINED,
			token.AUTORELEASING, token.NONNULL, token.NULLABLE,
			token.NULL_UNSPECIFIED:
			out = append(out, p.keywordSpec())
		case token.BLOCK:
			// __block written after the caret in a block pointer declarator.
			out = append(out, p.keywordSpec())
		case token.PTRAUTH:
			out = append(out, p.parsePtrauth())
		case token.ATTRIBUTE:
			// Attributes placed after pointer qualifiers (e.g. CF_RETURNS_RETAINED).
			attrs := p.parseAttrSpecList()
			if len(attrs) == 0 {
				return out
			}
			out = append(out, &ast.AttrSpec{
				Span:  ast.Span{Lo: attrs[0].Pos(), Hi: attrs[len(attrs)-1].End()},
				Attrs: attrs,
			})
		case token.IDENT:
			if p.inMethodType {
				if k, ok := nullabilityWord(p.text()); ok {
					t := p.tok()
					p.next()
					out = append(out, &ast.KeywordSpec{
						Span: ast.Span{Lo: t.Pos, Hi: t.End}, Kind: k})
					continue
				}
			}
			return out
		default:
			return out
		}
	}
}

func (p *parser) startsDeclaratorTail(m dmode) bool {
	switch p.kind() {
	case token.MUL, token.XOR, token.LPAREN, token.LBRACK:
		return true
	case token.IDENT:
		return m != dmodeAbstract
	}
	return false
}

func (p *parser) parseDirectDeclarator(m dmode, lo token.Pos) ast.Declarator {
	var d ast.Declarator
	switch {
	case p.at(token.IDENT) && m != dmodeAbstract:
		id := p.ident()
		d = &ast.NameDeclarator{Span: ast.Span{Lo: id.Lo, Hi: id.Hi}, Ident: id}

	case p.at(token.LPAREN) && p.parenIsGrouping(m):
		pd := &ast.ParenDeclarator{Lparen: p.pos()}
		p.next()
		pd.Inner = p.parseDeclarator(m)
		pd.Rparen = p.expect(token.RPAREN)
		pd.Span = p.span(lo)
		d = pd
	}

	// Suffixes: [ … ] and ( … ), left to right.
	for {
		switch p.kind() {
		case token.LBRACK:
			d = p.parseArraySuffix(d, lo)
		case token.LPAREN:
			d = p.parseFuncSuffix(d, lo)
		default:
			if d == nil {
				if m == dmodeNormal {
					p.errHere("expected a declarator")
					return &ast.BadDeclarator{Span: p.span(lo)}
				}
				return nil // an abstract declarator may be absent entirely
			}
			p.parseDeclaratorTail()
			return d
		}
	}
}

// parseDeclaratorTail consumes what §5.7 allows after a declarator — an
// attribute list — and the assembler label that is not in the grammar but is
// in <sys/cdefs.h>, where __DARWIN_ALIAS renames most of libc.
func (p *parser) parseDeclaratorTail() {
	for {
		switch {
		case p.at(token.ATTRIBUTE):
			p.declAttrs = append(p.declAttrs, p.parseAttrSpecList()...)
		case p.at(token.ASM):
			p.next()
			p.expect(token.LPAREN)
			if p.at(token.STRING_LIT) {
				p.asmLabel = p.parseStringRun()
			} else {
				p.errHere("expected a string literal naming the symbol")
			}
			p.expect(token.RPAREN)
		default:
			return
		}
	}
}

// parenIsGrouping decides whether a '(' after nothing opens a grouping
// declarator or a parameter list. `(void)` and `(int, int)` are parameters;
// `(*f)` and `(^b)` group.
// parenIsGrouping decides whether a '(' opens a parenthesized declarator or a
// parameter list — the one genuine ambiguity in §5.7's declarator grammar.
//
// The answer is in the token after it, except that an attribute may stand
// between: `void (__attribute__((noescape)) ^)(NSUInteger)` is a block
// pointer, and a parameter list may begin with an attribute too. Skipping the
// attribute run and asking the same question of what follows is what tells
// them apart, and is what clang does.
func (p *parser) parenIsGrouping(m dmode) bool {
	switch t := p.peekTok(p.afterAttrs(1)); t.Kind {
	case token.MUL, token.XOR, token.LPAREN:
		return true
	case token.IDENT:
		return m != dmodeAbstract && !p.isTypeName(p.name(t))
	}
	return false
}

// afterAttrs is the lookahead offset of the first token past an attribute run
// beginning at offset i. It only counts parentheses, because an attribute's
// arguments are §8's BalancedTokenSequence and nothing here has to understand
// them.
func (p *parser) afterAttrs(i int) int {
	for p.peekTok(i).Kind == token.ATTRIBUTE {
		j := i + 1
		depth := 0
		for {
			switch p.peekTok(j).Kind {
			case token.LPAREN:
				depth++
			case token.RPAREN:
				depth--
			case token.EOF:
				return i
			}
			j++
			if depth == 0 {
				break
			}
		}
		i = j
	}
	return i
}

func (p *parser) parseArraySuffix(inner ast.Declarator, lo token.Pos) ast.Declarator {
	a := &ast.ArrayDeclarator{Inner: inner, Lbrack: p.pos()}
	p.next()
	for {
		if p.at(token.STATIC) && !a.Static.IsValid() {
			a.Static = p.pos()
			p.next()
			continue
		}
		if q := p.parseQualList(); len(q) > 0 {
			a.Quals = append(a.Quals, q...)
			continue
		}
		break
	}
	switch {
	case p.at(token.RBRACK):
	case p.at(token.MUL) && p.peekTok(1).Kind == token.RBRACK:
		a.Star = p.pos()
		p.next()
	default:
		a.Len = p.parseAssign()
	}
	a.Rbrack = p.expect(token.RBRACK)
	a.Span = p.span(lo)
	return a
}

func (p *parser) parseFuncSuffix(inner ast.Declarator, lo token.Pos) ast.Declarator {
	f := &ast.FuncDeclarator{Inner: inner, Lparen: p.pos()}
	p.next()
	if p.at(token.RPAREN) {
		f.Rparen = p.pos()
		p.next()
		f.Span = p.span(lo)
		return f
	}
	// A K&R identifier list: bare names, none of them a type.
	if p.at(token.IDENT) && !p.isTypeName(p.text()) {
		for p.at(token.IDENT) {
			f.Idents = append(f.Idents, p.ident())
			if !p.at(token.COMMA) {
				break
			}
			p.next()
		}
	} else {
		p.parseParamList(&f.Params, &f.Ellipsis)
	}
	f.Rparen = p.expect(token.RPAREN)
	f.Span = p.span(lo)
	return f
}

// parseParamList reads a ParameterTypeList: declarations, then an optional
// `, ...`.
func (p *parser) parseParamList(params *[]*ast.ParamDecl, ellipsis *token.Pos) {
	for !p.at(token.RPAREN) && !p.at(token.EOF) {
		if p.at(token.ELLIPSIS) {
			*ellipsis = p.pos()
			p.next()
			break
		}
		start := p.i
		*params = append(*params, p.parseParamDecl())
		if p.i == start {
			p.advanceTo(parenFollow)
			break
		}
		if !p.at(token.COMMA) {
			break
		}
		p.next()
	}
}

func (p *parser) parseParamDecl() *ast.ParamDecl {
	lo := p.pos()
	d := &ast.ParamDecl{Specs: p.parseDeclSpecs(false)}
	d.Decl = p.parseDeclarator(dmodeEither)
	p.takeDeclAttrs()
	p.takeAsmLabel()
	d.Span = p.span(lo)
	return d
}

// parseTypeName reads §5.7's TypeName: a specifier-qualifier list and an
// optional abstract declarator.
func (p *parser) parseTypeName() *ast.TypeName {
	lo := p.pos()
	t := &ast.TypeName{Specs: p.parseDeclSpecs(true)}
	if len(t.Specs) == 0 {
		p.errHere("expected a type name")
	}
	t.Decl = p.parseDeclarator(dmodeAbstract)
	p.takeDeclAttrs()
	t.Span = p.span(lo)
	return t
}

// ---- initializers (§5.10) ----

func (p *parser) parseInitializer() ast.Expr {
	if p.at(token.LBRACE) {
		return p.parseInitList()
	}
	return p.parseAssign()
}

func (p *parser) parseInitList() *ast.InitList {
	lo := p.pos()
	l := &ast.InitList{Lbrace: p.expect(token.LBRACE)}
	for !p.at(token.RBRACE) && !p.at(token.EOF) {
		start := p.i
		l.Items = append(l.Items, p.parseInitItem())
		if p.i == start {
			p.advanceTo(declFollow)
			break
		}
		if !p.at(token.COMMA) {
			break
		}
		comma := p.pos()
		p.next()
		if p.at(token.RBRACE) {
			l.Comma = comma
		}
	}
	l.Rbrace = p.expect(token.RBRACE)
	l.Span = p.span(lo)
	return l
}

func (p *parser) parseInitItem() *ast.InitItem {
	lo := p.pos()
	it := &ast.InitItem{}
	for p.at(token.PERIOD) || (p.at(token.LBRACK) && p.designatorAhead()) {
		dlo := p.pos()
		if p.at(token.LBRACK) {
			d := &ast.IndexDesignator{Lbrack: p.pos()}
			p.next()
			d.Index = p.parseCond()
			d.Rbrack = p.expect(token.RBRACK)
			d.Span = p.span(dlo)
			it.Designators = append(it.Designators, d)
		} else {
			d := &ast.FieldDesignator{Dot: p.pos()}
			p.next()
			d.Name = p.expectName()
			d.Span = p.span(dlo)
			it.Designators = append(it.Designators, d)
		}
	}
	if len(it.Designators) > 0 {
		it.Assign = p.expect(token.ASSIGN)
	}
	it.Value = p.parseInitializer()
	it.Span = p.span(lo)
	return it
}

// designatorAhead reports whether the bracket the parser is looking at opens
// a designator list rather than a message send.
//
// The two are spelled the same way in the one place both may stand:
//
//	int a[] = { [2] = 7 };                  a designator
//	NSString *b[] = { [@"x" uppercaseString] };   a send
//
// and reading it as a designator gave "expected ']'" on the selector, which
// is a syntax error reported at a line with no syntax error in it. §6.7.9
// says what tells them apart: a designator list ends at an `=`, and nothing
// else in the grammar of an initializer does. So the whole chain is scanned
// -- brackets balanced, `.name` steps taken -- and what decides is the token
// after it. An unbalanced bracket is not a designator either; the expression
// parser reports it, which is where the error is.
func (p *parser) designatorAhead() bool {
	i := p.i
	seen := false
	for i < len(p.toks) {
		switch p.toks[i].Kind {
		case token.LBRACK:
			depth := 0
			for ; i < len(p.toks); i++ {
				switch p.toks[i].Kind {
				case token.LBRACK:
					depth++
				case token.RBRACK:
					depth--
				case token.EOF:
					return false
				}
				if depth == 0 {
					break
				}
			}
			if i >= len(p.toks) {
				return false
			}
			i++ // past the ]
			seen = true
		case token.PERIOD:
			if i+1 >= len(p.toks) || p.toks[i+1].Kind != token.IDENT {
				return false
			}
			i += 2
			seen = true
		default:
			return seen && p.toks[i].Kind == token.ASSIGN
		}
	}
	return false
}

// ---- name bookkeeping ----

// declareDeclarator enters a declarator's name in the scope it belongs to.
//
// A C declaration written inside an @interface body belongs to *file* scope.
// §4 puts nothing but methods, properties and instance variables in a class's
// own namespace, and Foundation relies on it:
//
//	@interface NSString (Transform)
//	typedef NSString *NSStringTransform;
//	- (id)stringByApplyingTransform:(NSStringTransform)t;
//	@end
//
// with the typedef used again in a later category. The scope an @interface
// opens exists for its type parameters, and a typedef that fell into it would
// leave with the @end.
func (p *parser) declareDeclarator(d ast.Declarator, isTypedef bool) {
	id := declNameOf(d)
	if id == nil {
		return
	}
	k := nameOrdinary
	if isTypedef {
		k = nameTypedef
	}
	if p.inClassScope() {
		p.declareGlobal(id.Name(p.f), k)
		return
	}
	p.declare(id.Name(p.f), k)
}

// declareParams enters a function's parameters in the scope its body will be
// parsed in, so that a parameter shadowing a type name is read as a name.
func (p *parser) declareParams(d ast.Declarator) {
	fd, ok := d.(*ast.FuncDeclarator)
	if !ok {
		// The declarator may be `*f(void)` or `(*f)(void)`; walk inward.
		switch inner := d.(type) {
		case *ast.PtrDeclarator:
			p.declareParams(inner.Inner)
		case *ast.BlockPtrDeclarator:
			p.declareParams(inner.Inner)
		case *ast.ParenDeclarator:
			p.declareParams(inner.Inner)
		}
		return
	}
	for _, prm := range fd.Params {
		if prm.Decl != nil {
			if id := prm.Decl.DeclName(); id != nil {
				p.declare(id.Name(p.f), nameOrdinary)
			}
		}
	}
	for _, id := range fd.Idents {
		p.declare(id.Name(p.f), nameOrdinary)
	}
}

func declNameOf(d ast.Declarator) *ast.Ident {
	if d == nil {
		return nil
	}
	return d.DeclName()
}

func hasKeyword(specs ast.DeclSpecs, k token.Kind) bool {
	for _, s := range specs {
		if ks, ok := s.(*ast.KeywordSpec); ok && ks.Kind == k {
			return true
		}
	}
	return false
}
