package parser

import (
	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/token"
)

// parseObjCDecl dispatches the six §4 forms plus the three of §4.4. attrs are
// the attributes written before the directive, which §4.1 and §4.3 allow and
// §5.9 forbids between the keyword and the name.
func (p *parser) parseObjCDecl(lo token.Pos, attrs []*ast.Attr) ast.Decl {
	switch p.kind() {
	case token.AT_INTERFACE:
		return p.parseInterface(lo, attrs)
	case token.AT_IMPLEMENTATION:
		return p.parseImplementation(lo, attrs)
	case token.AT_PROTOCOL:
		return p.parseProtocol(lo, attrs)
	case token.AT_CLASS:
		return p.parseClassForward(lo)
	case token.AT_COMPATIBILITY_ALIAS:
		return p.parseCompatAlias(lo)
	case token.AT_IMPORT:
		return p.parseModuleImport(lo)
	}
	p.errHere("expected a declaration")
	return &ast.BadDecl{Span: p.span(lo)}
}

// parseInterface reads §4.1's @interface, §4.2's category, and §4.2's class
// extension. All three open the same way — `@interface Name` — and what
// follows the name decides which of them it is.
func (p *parser) parseInterface(lo token.Pos, attrs []*ast.Attr) ast.Decl {
	keyword := p.pos()
	p.next()
	name := p.expectIdent()
	if name == nil {
		p.advanceTo(declFollow)
		return &ast.BadDecl{Span: p.span(lo)}
	}
	p.declareGlobal(name.Name(p.f), nameClass)

	// Type parameters and the ivars and members after them are read in a
	// scope of their own, so that `T` names a type inside the class and
	// nothing outside it.
	p.pushClassScope()
	defer p.popClassScope()

	var typeParams *ast.TypeParamList
	if p.at(token.LSS) && p.angleIsTypeParamList() {
		typeParams = p.parseTypeParamList()
		p.recordClassParams(name, typeParams)
	}

	if p.at(token.LPAREN) {
		if typeParams == nil {
			// A category on a generic class may use the class's parameters
			// without restating them.
			p.declareClassParams(name)
		}
		return p.parseCategory(lo, keyword, attrs, name, typeParams)
	}

	d := &ast.ClassInterfaceDecl{Attrs: attrs, Keyword: keyword, Name: name, TypeParams: typeParams}
	if p.at(token.COLON) {
		d.Colon = p.pos()
		p.next()
		d.Super = p.expectIdent()
		if d.Super != nil {
			// §4.1: the list after a superclass is type arguments if the
			// superclass is generic and a protocol list otherwise, and both
			// may appear in that order.
			if p.at(token.LSS) && !p.angleIsProtocolList() {
				d.SuperArgs = p.parseTypeArgList()
			}
		}
	}
	if p.at(token.LSS) {
		d.Protocols = p.parseProtocolRefList()
	}
	if p.at(token.LBRACE) {
		d.Ivars = p.parseIvarList()
	}
	d.Members = p.parseMembers(false)
	d.EndKeyword = p.expectEnd()
	d.Span = p.span(lo)
	return d
}

// parseCategory reads a category interface, or a class extension when the
// parentheses are empty (§4.2).
func (p *parser) parseCategory(lo, keyword token.Pos, attrs []*ast.Attr,
	class *ast.Ident, typeParams *ast.TypeParamList) ast.Decl {

	d := &ast.CategoryDecl{Attrs: attrs, Keyword: keyword, Class: class,
		TypeParams: typeParams, Lparen: p.pos()}
	p.next()
	if !p.at(token.RPAREN) {
		d.Name = p.expectIdent()
	}
	d.Rparen = p.expect(token.RPAREN)
	if p.at(token.LSS) {
		d.Protocols = p.parseProtocolRefList()
	}
	if p.at(token.LBRACE) {
		// Only an extension may declare instance variables; a category with
		// a brace here is reported by the analyzer, which knows the rule.
		d.Ivars = p.parseIvarList()
	}
	d.Members = p.parseMembers(false)
	d.EndKeyword = p.expectEnd()
	d.Span = p.span(lo)
	return d
}

// parseImplementation reads §4.1's @implementation and §4.2's category
// implementation.
func (p *parser) parseImplementation(lo token.Pos, attrs []*ast.Attr) ast.Decl {
	keyword := p.pos()
	p.next()
	name := p.expectIdent()
	if name == nil {
		p.advanceTo(declFollow)
		return &ast.BadDecl{Span: p.span(lo)}
	}
	p.declareGlobal(name.Name(p.f), nameClass)
	p.pushClassScope()
	defer p.popClassScope()
	p.declareClassParams(name)

	if p.at(token.LPAREN) {
		d := &ast.CategoryImplDecl{Keyword: keyword, Class: name, Lparen: p.pos()}
		p.next()
		d.Name = p.expectIdent()
		d.Rparen = p.expect(token.RPAREN)
		d.Members = p.parseMembers(true)
		d.EndKeyword = p.expectEnd()
		d.Span = p.span(lo)
		return d
	}

	d := &ast.ClassImplDecl{Attrs: attrs, Keyword: keyword, Name: name}
	if p.at(token.COLON) {
		d.Colon = p.pos()
		p.next()
		d.Super = p.expectIdent()
	}
	if p.at(token.LBRACE) {
		d.Ivars = p.parseIvarList()
	}
	d.Members = p.parseMembers(true)
	d.EndKeyword = p.expectEnd()
	d.Span = p.span(lo)
	return d
}

// parseProtocol reads §4.3's protocol declaration, or §4.4's forward
// declaration list. They are told apart by what follows the first name: a
// semicolon or a comma means the forward form.
func (p *parser) parseProtocol(lo token.Pos, attrs []*ast.Attr) ast.Decl {
	keyword := p.pos()
	p.next()
	if !p.at(token.IDENT) {
		p.errHere("expected a protocol name")
		p.advanceTo(declFollow)
		return &ast.BadDecl{Span: p.span(lo)}
	}
	switch p.peekTok(1).Kind {
	case token.SEMI, token.COMMA:
		d := &ast.ProtocolForwardDecl{Keyword: keyword}
		for {
			id := p.expectIdent()
			if id == nil {
				break
			}
			p.declareGlobal(id.Name(p.f), nameProtocol)
			d.Names = append(d.Names, id)
			if !p.at(token.COMMA) {
				break
			}
			p.next()
		}
		d.Semi = p.expectSemi()
		d.Span = p.span(lo)
		return d
	}

	d := &ast.ProtocolDecl{Attrs: attrs, Keyword: keyword, Name: p.ident()}
	p.declareGlobal(d.Name.Name(p.f), nameProtocol)
	if p.at(token.LSS) {
		d.Protocols = p.parseProtocolRefList()
	}
	p.pushClassScope()
	d.Members = p.parseMembers(false)
	p.popClassScope()
	d.EndKeyword = p.expectEnd()
	d.Span = p.span(lo)
	return d
}

// parseClassForward reads §4.4's `@class A, B<T>;`.
func (p *parser) parseClassForward(lo token.Pos) ast.Decl {
	d := &ast.ClassForwardDecl{Keyword: p.pos()}
	p.next()
	for {
		flo := p.pos()
		id := p.expectIdent()
		if id == nil {
			p.advanceTo(declFollow)
			break
		}
		p.declareGlobal(id.Name(p.f), nameClass)
		fc := &ast.ForwardClass{Name: id}
		if p.at(token.LSS) && p.angleIsTypeParamList() {
			// A forward declaration may state a generic class's parameters.
			// They are scoped to this declaration, and remembered for the
			// @implementation that will need them back.
			p.pushScope()
			fc.TypeParams = p.parseTypeParamList()
			p.popScope()
			p.recordClassParams(id, fc.TypeParams)
		}
		fc.Span = p.span(flo)
		d.Names = append(d.Names, fc)
		if !p.at(token.COMMA) {
			break
		}
		p.next()
	}
	d.Semi = p.expectSemi()
	d.Span = p.span(lo)
	return d
}

func (p *parser) parseCompatAlias(lo token.Pos) ast.Decl {
	d := &ast.CompatAliasDecl{Keyword: p.pos()}
	p.next()
	d.Alias = p.expectIdent()
	d.Class = p.expectIdent()
	if d.Alias != nil {
		// The alias names the same class, so it is a class name too.
		p.declareGlobal(d.Alias.Name(p.f), nameClass)
	}
	d.Semi = p.expectSemi()
	d.Span = p.span(lo)
	return d
}

// parseModuleImport reads §4.4's `@import Foundation.NSString;`.
func (p *parser) parseModuleImport(lo token.Pos) ast.Decl {
	d := &ast.ImportDecl{Keyword: p.pos()}
	p.next()
	for {
		id := p.expectIdent()
		if id == nil {
			p.advanceTo(declFollow)
			break
		}
		d.Path = append(d.Path, id)
		if !p.at(token.PERIOD) {
			break
		}
		p.next()
	}
	d.Semi = p.expectSemi()
	d.Span = p.span(lo)
	return d
}

func (p *parser) expectEnd() token.Pos {
	if p.at(token.AT_END) {
		pos := p.pos()
		p.next()
		return pos
	}
	p.errHere("expected '@end'")
	return token.NoPos
}

// ---- members (§4.6) ----

// parseMembers reads an InterfaceDeclarationList or an
// ImplementationDefinitionList. The two differ in what they admit — a method
// body, a function definition and a property implementation belong to the
// second — and impl says which is being read.
func (p *parser) parseMembers(impl bool) []ast.Decl {
	var out []ast.Decl
	for !p.at(token.AT_END) && !p.at(token.EOF) {
		start := p.i
		out = append(out, p.parseMember(impl))
		if p.i == start {
			p.advanceTo(memberFollow)
			if p.at(token.SEMI) {
				p.next()
			}
			if p.i == start {
				p.next()
			}
		}
	}
	return out
}

func (p *parser) parseMember(impl bool) ast.Decl {
	lo := p.pos()
	switch p.kind() {
	case token.SEMI:
		semi := p.pos()
		p.next()
		return &ast.EmptyDecl{Span: p.span(lo), Semi: semi}

	case token.ADD, token.SUB:
		return p.parseMethod(lo, impl)

	case token.AT_PROPERTY:
		return p.parseProperty(lo)

	case token.AT_SYNTHESIZE, token.AT_DYNAMIC:
		return p.parsePropertyImpl(lo)

	case token.AT_REQUIRED, token.AT_OPTIONAL:
		d := &ast.RequirementDecl{Keyword: p.pos(), Kind: p.kind()}
		p.next()
		d.Span = p.span(lo)
		return d

	case token.AT_END:
		p.errHere("expected a member")
		return &ast.BadDecl{Span: p.span(lo)}

	case token.HASH:
		p.skipDirectiveLine()
		return &ast.EmptyDecl{Span: p.span(lo)}
	}

	// Everything else is an ordinary C declaration, and in an
	// @implementation it may be a function definition: §4.6 grants one
	// written there access to the class's instance variables.
	if impl {
		return p.parseDeclOrFunc(lo, nil)
	}
	return p.parseDeclaration()
}

// ---- instance variables (§4.5) ----

func (p *parser) parseIvarList() *ast.IvarList {
	lo := p.pos()
	l := &ast.IvarList{Lbrace: p.pos()}
	p.next()
	for !p.at(token.RBRACE) && !p.at(token.EOF) {
		start := p.i
		switch p.kind() {
		case token.AT_PRIVATE, token.AT_PROTECTED, token.AT_PUBLIC, token.AT_PACKAGE:
			vlo := p.pos()
			v := &ast.VisibilityDecl{Keyword: p.pos(), Kind: p.kind()}
			p.next()
			v.Span = p.span(vlo)
			l.Items = append(l.Items, v)
		case token.HASH:
			p.skipDirectiveLine()
		default:
			l.Items = append(l.Items, p.parseStructDeclaration())
		}
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
	l.Rbrace = p.expect(token.RBRACE)
	l.Span = p.span(lo)
	return l
}

// ---- methods (§4.7) ----

// parseMethod reads a method declaration, and its definition when a body
// follows instead of a semicolon.
func (p *parser) parseMethod(lo token.Pos, impl bool) ast.Decl {
	d := &ast.MethodDecl{Keyword: p.pos(), Kind: p.kind()}
	p.next()
	if p.at(token.LPAREN) {
		d.Type = p.parseMethodType()
	}
	if p.at(token.ATTRIBUTE) {
		d.Attrs = p.parseAttrSpecList()
	}

	// The selector. A unary method is one piece with no colon; anything else
	// is a run of keyword declarators.
	p.pushScope()
	if isSelector(p.tok()) && p.peekTok(1).Kind != token.COLON {
		d.Sel = p.ident()
	} else {
		for isSelector(p.tok()) || p.at(token.COLON) {
			k := p.parseKeywordDecl()
			if k == nil {
				break
			}
			d.Parts = append(d.Parts, k)
		}
		if len(d.Parts) == 0 {
			p.errHere("expected a selector")
		}
		// §4.7's MethodParameterSuffix: the C-style trailing parameters
		// that make a method variadic.
		for p.at(token.COMMA) {
			p.next()
			if p.at(token.ELLIPSIS) {
				d.Ellipsis = p.pos()
				p.next()
				break
			}
			start := p.i
			d.Params = append(d.Params, p.parseParamDecl())
			if p.i == start {
				break
			}
		}
	}

	if p.at(token.ATTRIBUTE) {
		d.TailAttrs = p.parseAttrSpecList()
	}

	switch {
	case p.at(token.LBRACE):
		if p.mode&SkipBodies != 0 {
			d.Body = p.skipBody()
		} else {
			d.Body = p.parseCompound(false)
		}
	case impl && p.isDeclSpecStart(p.tok()) && !p.at(token.SEMI):
		// The obsolescent form: parameter declarations after the selector.
		for p.isDeclSpecStart(p.tok()) && !p.at(token.LBRACE) && !p.at(token.EOF) {
			start := p.i
			if g, ok := p.parseDeclaration().(*ast.GenDecl); ok {
				d.KR = append(d.KR, g)
			}
			if p.i == start {
				break
			}
		}
		if p.at(token.LBRACE) {
			d.Body = p.parseCompound(false)
		}
	default:
		d.Semi = p.expectSemi()
	}
	p.popScope()
	d.Span = p.span(lo)
	return d
}

// parseKeywordDecl reads one `[Selector] : [MethodType] [Attrs] Identifier`.
func (p *parser) parseKeywordDecl() *ast.KeywordDecl {
	lo := p.pos()
	k := &ast.KeywordDecl{}
	if isSelector(p.tok()) && p.peekTok(1).Kind == token.COLON {
		k.Sel = p.ident()
	}
	if !p.at(token.COLON) {
		p.errHere("expected ':' in the method selector")
		return nil
	}
	k.Colon = p.pos()
	p.next()
	if p.at(token.LPAREN) {
		k.Type = p.parseMethodType()
	}
	if p.at(token.ATTRIBUTE) {
		k.Attrs = p.parseAttrSpecList()
	}
	k.Name = p.expectIdent()
	if k.Name != nil {
		p.declare(k.Name.Name(p.f), nameOrdinary)
	}
	k.Span = p.span(lo)
	return k
}

// parseMethodType reads §4.7's `( {ProtocolQualifier} [TypeName] )`.
//
// inMethodType is set while the parentheses are open because this is where
// §5.6's underscore-free nullability spellings mean the qualifier: `nullable`
// is a qualifier here and an ordinary identifier everywhere else.
func (p *parser) parseMethodType() *ast.MethodType {
	lo := p.pos()
	m := &ast.MethodType{Lparen: p.pos()}
	p.next()

	outer := p.inMethodType
	p.inMethodType = true
	defer func() { p.inMethodType = outer }()

	for p.at(token.IDENT) {
		q, ok := protoQualOf(p.text())
		if !ok {
			break
		}
		qlo := p.pos()
		p.next()
		m.Quals = append(m.Quals, &ast.ProtoQual{Span: p.span(qlo), Kind: q})
	}
	if !p.at(token.RPAREN) {
		m.Type = p.parseTypeName()
	}
	m.Rparen = p.expect(token.RPAREN)
	m.Span = p.span(lo)
	return m
}

func protoQualOf(name string) (ast.ProtoQualKind, bool) {
	switch name {
	case "in":
		return ast.QualIn, true
	case "out":
		return ast.QualOut, true
	case "inout":
		return ast.QualInout, true
	case "bycopy":
		return ast.QualBycopy, true
	case "byref":
		return ast.QualByref, true
	case "oneway":
		return ast.QualOneway, true
	}
	return 0, false
}

// ---- properties (§4.8) ----

func (p *parser) parseProperty(lo token.Pos) ast.Decl {
	d := &ast.PropertyDecl{Keyword: p.pos()}
	p.next()
	if p.at(token.LPAREN) {
		d.Lparen = p.pos()
		p.next()
		for !p.at(token.RPAREN) && !p.at(token.EOF) {
			a := p.parsePropertyAttr()
			if a == nil {
				p.advanceTo(parenFollow)
				break
			}
			d.Attrs = append(d.Attrs, a)
			if !p.at(token.COMMA) {
				break
			}
			p.next()
		}
		d.Rparen = p.expect(token.RPAREN)
	}
	d.Specs = p.parseDeclSpecs(true)
	for {
		start := p.i
		dcl := p.parseDeclarator(dmodeNormal)
		p.takeDeclAttrs()
		p.takeAsmLabel()
		d.List = append(d.List, dcl)
		if p.i == start || !p.at(token.COMMA) {
			break
		}
		p.next()
	}
	d.Semi = p.expectSemi()
	d.Span = p.span(lo)
	return d
}

// parsePropertyAttr reads one entry of §4.8's closed set, or a getter or
// setter naming a selector. An identifier that is none of them is a syntax
// error, which is what §4.8 says it is.
func (p *parser) parsePropertyAttr() *ast.PropertyAttr {
	lo := p.pos()
	if !isName(p.tok()) {
		p.errHere("expected a property attribute")
		return nil
	}
	name := p.text()
	kind, ok := propertyAttrOf(name)
	if !ok {
		p.errHere("unknown property attribute '" + name + "'")
		return nil
	}
	a := &ast.PropertyAttr{Kind: kind, Name: p.ident()}
	if kind == ast.PropGetter || kind == ast.PropSetter {
		a.Assign = p.expect(token.ASSIGN)
		a.Sel = p.expectName()
		if kind == ast.PropSetter {
			// The trailing colon is part of the setter's selector, which is
			// why §4.8 writes it into the production.
			a.Colon = p.expect(token.COLON)
		}
	}
	a.Span = p.span(lo)
	return a
}

func propertyAttrOf(name string) (ast.PropertyAttrKind, bool) {
	switch name {
	case "class":
		return ast.PropClass, true
	case "direct":
		return ast.PropDirect, true
	case "atomic":
		return ast.PropAtomic, true
	case "nonatomic":
		return ast.PropNonatomic, true
	case "readonly":
		return ast.PropReadonly, true
	case "readwrite":
		return ast.PropReadwrite, true
	case "assign":
		return ast.PropAssign, true
	case "retain":
		return ast.PropRetain, true
	case "copy":
		return ast.PropCopy, true
	case "strong":
		return ast.PropStrong, true
	case "weak":
		return ast.PropWeak, true
	case "unsafe_unretained":
		return ast.PropUnsafeUnretained, true
	case "nullable":
		return ast.PropNullable, true
	case "nonnull":
		return ast.PropNonnull, true
	case "null_resettable":
		return ast.PropNullResettable, true
	case "null_unspecified":
		return ast.PropNullUnspecified, true
	case "getter":
		return ast.PropGetter, true
	case "setter":
		return ast.PropSetter, true
	}
	return 0, false
}

// parsePropertyImpl reads §4.8's @synthesize and @dynamic.
func (p *parser) parsePropertyImpl(lo token.Pos) ast.Decl {
	d := &ast.PropertyImplDecl{Keyword: p.pos(), Kind: p.kind()}
	p.next()
	for {
		ilo := p.pos()
		it := &ast.PropertyImplItem{Name: p.expectIdent()}
		if it.Name == nil {
			p.advanceTo(declFollow)
			break
		}
		if p.at(token.ASSIGN) {
			it.Assign = p.pos()
			p.next()
			it.Ivar = p.expectIdent()
		}
		it.Span = p.span(ilo)
		d.Items = append(d.Items, it)
		if !p.at(token.COMMA) {
			break
		}
		p.next()
	}
	d.Semi = p.expectSemi()
	d.Span = p.span(lo)
	return d
}

// recordClassParams remembers a generic class's parameter names, and
// declareClassParams puts them back in scope.
//
// §4.1 gives an @implementation no TypeParameterList — generics are erased
// and exist only in the interface — but the methods it defines are written
// in terms of the parameters the interface declared, and `- (V)objectForKey:(K)key`
// has to parse. clang injects them the same way, and for the same reason.
func (p *parser) recordClassParams(class *ast.Ident, l *ast.TypeParamList) {
	if class == nil || l == nil || len(l.Params) == 0 {
		return
	}
	if p.classParams == nil {
		p.classParams = map[string][]string{}
	}
	var names []string
	for _, tp := range l.Params {
		if tp.Name != nil {
			names = append(names, tp.Name.Name(p.f))
		}
	}
	p.classParams[class.Name(p.f)] = names
}

func (p *parser) declareClassParams(class *ast.Ident) {
	if class == nil {
		return
	}
	for _, n := range p.classParams[class.Name(p.f)] {
		p.declare(n, nameTypeParam)
	}
}
