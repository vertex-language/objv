package parser

import (
	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/token"
)

// parseMessage reads §6.3's `[ Receiver MessageSelector ]`.
//
// The receiver is an ordinary expression, with two shapes that are not:
// `super`, which is a receiver and nothing else, and a class name carrying a
// type argument list, which is an expression nowhere else in the language.
// Both are recognized here because here is the only place they are legal.
func (p *parser) parseMessage(lo token.Pos) ast.Expr {
	m := &ast.MessageExpr{Lbrack: p.pos()}
	p.next()
	m.Recv = p.parseReceiver()

	// A unary send is one selector piece and no colon.
	if isSelector(p.tok()) && p.peekTok(1).Kind != token.COLON {
		m.Sel = p.ident()
	} else {
		for {
			if !isSelector(p.tok()) && !p.at(token.COLON) {
				break
			}
			a := p.parseKeywordArg()
			if a == nil {
				break
			}
			m.Args = append(m.Args, a)
		}
		if len(m.Args) == 0 {
			p.errHere("expected a selector after the receiver")
			p.advanceTo(brackFollow)
		}
	}
	m.Rbrack = p.expect(token.RBRACK)
	m.Span = p.span(lo)
	return m
}

// parseReceiver reads §6.3's Receiver.
func (p *parser) parseReceiver() ast.Expr {
	lo := p.pos()
	if p.atWord("super") {
		p.next()
		return &ast.SuperExpr{Span: p.span(lo)}
	}
	// `[NSArray<NSString *> array]` — a class name with type arguments. The
	// bare class name is an ordinary identifier and needs nothing special;
	// only the angle brackets do, and only a class name may carry them here.
	if p.at(token.IDENT) && p.peekTok(1).Kind == token.LSS && p.isClassName(p.text()) {
		name := p.ident()
		args := p.parseTypeArgList()
		return &ast.ClassExpr{Span: p.span(lo), Name: name, TypeArgs: args}
	}
	return p.parseExpr()
}

// parseKeywordArg reads `[Selector] : AssignmentExpression {, AssignmentExpression}`.
//
// The trailing values are the arguments of a variadic method, which §6.3
// permits only on the final keyword; that is a constraint, and the analyzer
// is where it is checked.
func (p *parser) parseKeywordArg() *ast.KeywordArg {
	lo := p.pos()
	a := &ast.KeywordArg{}
	if isSelector(p.tok()) && p.peekTok(1).Kind == token.COLON {
		a.Sel = p.ident()
	}
	if !p.at(token.COLON) {
		p.errHere("expected ':' in the message selector")
		return nil
	}
	a.Colon = p.pos()
	p.next()
	for {
		start := p.i
		a.Vals = append(a.Vals, p.parseAssign())
		if p.i == start {
			p.advanceTo(brackFollow)
			break
		}
		if !p.at(token.COMMA) {
			break
		}
		p.next()
	}
	a.Span = p.span(lo)
	return a
}

// parseAtExpr reads what a bare '@' heads (§6.8): a boxed expression, an
// array literal, or a dictionary literal. The scanner has already taken the
// directives and @"…" — an '@' that reaches here is one of these three or a
// mistake.
func (p *parser) parseAtExpr(lo token.Pos) ast.Expr {
	at := p.pos()
	p.next()
	switch p.kind() {
	case token.LBRACK:
		a := &ast.ArrayLit{At: at, Lbrack: p.pos()}
		p.next()
		for !p.at(token.RBRACK) && !p.at(token.EOF) {
			start := p.i
			a.Elems = append(a.Elems, p.parseAssign())
			if p.i == start {
				p.advanceTo(brackFollow)
				break
			}
			if !p.at(token.COMMA) {
				break
			}
			comma := p.pos()
			p.next()
			if p.at(token.RBRACK) {
				a.Comma = comma // a trailing comma is allowed
			}
		}
		a.Rbrack = p.expect(token.RBRACK)
		a.Span = p.span(lo)
		return a

	case token.LBRACE:
		d := &ast.DictLit{At: at, Lbrace: p.pos()}
		p.next()
		for !p.at(token.RBRACE) && !p.at(token.EOF) {
			start := p.i
			klo := p.pos()
			kv := &ast.KeyValue{Key: p.parseAssign()}
			kv.Colon = p.expect(token.COLON)
			kv.Value = p.parseAssign()
			kv.Span = p.span(klo)
			d.Pairs = append(d.Pairs, kv)
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
				d.Comma = comma
			}
		}
		d.Rbrace = p.expect(token.RBRACE)
		d.Span = p.span(lo)
		return d

	case token.LPAREN:
		b := &ast.BoxedExpr{At: at, Lparen: p.pos()}
		p.next()
		b.X = p.parseExpr()
		b.Rparen = p.expect(token.RPAREN)
		b.Span = p.span(lo)
		return b

	case token.INT_LIT, token.FLOAT_LIT, token.CHAR_LIT, token.BOOL_LIT,
		token.ADD, token.SUB:
		// @42, @-1, @'c', @__objc_yes. The sign is an ordinary unary
		// expression, so the operand is parsed as one — which also gives
		// `@-x` a tree, for the analyzer to reject as not a constant.
		b := &ast.BoxedExpr{At: at, X: p.parseUnary()}
		b.Span = p.span(lo)
		return b
	}
	p.errHere("expected a boxed expression, @[…] or @{…} after '@'")
	return &ast.BadExpr{Span: p.span(lo)}
}

// parseSelectorExpr reads §6.4's @selector ( SelectorName ).
//
// A selector name is not an expression and not an identifier: it is a run of
// pieces and colons, any piece may be a keyword, and a piece may be missing
// entirely — @selector(a::) names a real method.
func (p *parser) parseSelectorExpr(lo token.Pos) ast.Expr {
	e := &ast.SelectorExpr{Keyword: p.pos()}
	p.next()
	e.Lparen = p.expect(token.LPAREN)
	for !p.at(token.RPAREN) && !p.at(token.EOF) {
		plo := p.pos()
		part := &ast.SelectorPart{}
		if isSelector(p.tok()) {
			part.Name = p.ident()
		}
		if p.at(token.COLON) {
			part.Colon = p.pos()
			p.next()
		} else if part.Name == nil {
			p.errHere("expected a selector name")
			p.advanceTo(parenFollow)
			break
		}
		part.Span = p.span(plo)
		e.Parts = append(e.Parts, part)
	}
	if len(e.Parts) == 0 {
		p.errHere("expected a selector name")
	}
	e.Rparen = p.expect(token.RPAREN)
	e.Span = p.span(lo)
	return e
}

// parseProtocolExpr reads §6.4's @protocol ( ProtocolName ).
func (p *parser) parseProtocolExpr(lo token.Pos) ast.Expr {
	e := &ast.ProtocolExpr{Keyword: p.pos()}
	p.next()
	e.Lparen = p.expect(token.LPAREN)
	e.Name = p.expectIdent()
	e.Rparen = p.expect(token.RPAREN)
	e.Span = p.span(lo)
	return e
}

// parseEncodeExpr reads §6.4's @encode ( TypeName ).
func (p *parser) parseEncodeExpr(lo token.Pos) ast.Expr {
	e := &ast.EncodeExpr{Keyword: p.pos()}
	p.next()
	e.Lparen = p.expect(token.LPAREN)
	e.Type = p.parseTypeName()
	e.Rparen = p.expect(token.RPAREN)
	e.Span = p.span(lo)
	return e
}

// parseAvailability reads §6.10's @available ( … , * ) and its
// __builtin_available spelling.
//
// A version tuple is not an expression — `10.12.1` is a pp-number no phase
// gives a value to — so the tokens are taken as an extent and the digits are
// read back out of it where they are needed.
func (p *parser) parseAvailability(lo token.Pos) ast.Expr {
	e := &ast.AvailabilityExpr{Keyword: p.pos(), Kind: p.kind()}
	p.next()
	e.Lparen = p.expect(token.LPAREN)
	for !p.at(token.RPAREN) && !p.at(token.EOF) {
		if p.at(token.MUL) {
			e.Star = p.pos()
			p.next()
			break
		}
		slo := p.pos()
		s := &ast.AvailabilitySpec{Platform: p.expectIdent()}
		if s.Platform == nil {
			p.advanceTo(parenFollow)
			break
		}
		vlo := p.pos()
		for p.at(token.INT_LIT) || p.at(token.FLOAT_LIT) || p.at(token.PERIOD) {
			p.next()
		}
		if p.prevEnd() <= vlo {
			p.errHere("expected a version after the platform name")
			p.advanceTo(parenFollow)
			break
		}
		s.Version = ast.Span{Lo: vlo, Hi: p.prevEnd()}
		s.Span = p.span(slo)
		e.Specs = append(e.Specs, s)
		if !p.at(token.COMMA) {
			break
		}
		e.Comma = p.pos()
		p.next()
	}
	if !e.Star.IsValid() {
		// §6.10 makes the trailing `, *` mandatory: it is what makes the
		// check succeed on every platform the list does not name, so a check
		// without one means something the author did not write.
		p.errHere("expected ', *' to close the availability check")
	}
	e.Rparen = p.expect(token.RPAREN)
	e.Span = p.span(lo)
	return e
}

// parseBlockLit reads §6.9's `^ [TypeName] [BlockParameters] CompoundStatement`.
//
// The optional return type is told from a parameter list by what follows the
// '^': a '(' may open either, so a type is read only when the token after the
// '(' cannot open a parameter declaration. `^(int x){…}` takes an int;
// `^(int){…}` — a block returning int, taking nothing — is the same three
// tokens, and §6.9 resolves it as a parameter list of one unnamed int, which
// is what clang does.
func (p *parser) parseBlockLit(lo token.Pos) ast.Expr {
	b := &ast.BlockLit{Caret: p.pos()}
	p.next()

	// An explicit return type: anything that opens a type name and is not
	// the parameter list's own parenthesis.
	if p.isTypeSpecStart(p.tok()) && !p.at(token.LPAREN) {
		b.Type = p.parseBlockReturnType()
	}
	if p.at(token.LPAREN) {
		b.Lparen = p.pos()
		p.next()
		p.parseParamList(&b.Params, &b.Ellipsis)
		b.Rparen = p.expect(token.RPAREN)
	}
	if p.at(token.LBRACE) {
		b.Body = p.parseBlockBody(b.Params)
	} else {
		p.errHere("expected '{' to open the block's body")
	}
	b.Span = p.span(lo)
	return b
}

// parseBlockReturnType reads the optional TypeName of §6.9's block literal.
//
// It is not parseTypeName, and the difference is the parameter list: in
// `^int(int x){…}` an ordinary type name would read `int(int x)` as a
// function type and leave the block with no parameters and a return type
// nobody wrote. §6.9 factors the production as `[TypeName]
// [BlockParameters]` and notes that the factoring covers the common
// spelling rather than every equivalent one — so the return type is read
// here as specifiers and pointers, and the first '(' belongs to the
// parameters.
func (p *parser) parseBlockReturnType() *ast.TypeName {
	lo := p.pos()
	t := &ast.TypeName{Specs: p.parseDeclSpecs(true)}
	// Pointers and block pointers derive a type; a bracket or a
	// parenthesis does not, here.
	for p.at(token.MUL) || p.at(token.XOR) {
		dlo := p.pos()
		if p.at(token.MUL) {
			d := &ast.PtrDeclarator{Star: p.pos()}
			p.next()
			d.Quals = p.parseQualList()
			d.Inner = t.Decl
			d.Span = p.span(dlo)
			t.Decl = d
			continue
		}
		d := &ast.BlockPtrDeclarator{Caret: p.pos()}
		p.next()
		d.Quals = p.parseQualList()
		d.Inner = t.Decl
		d.Span = p.span(dlo)
		t.Decl = d
	}
	t.Span = p.span(lo)
	return t
}

// parseBlockBody parses the body with the parameters in scope, the way a
// function body is parsed.
func (p *parser) parseBlockBody(params []*ast.ParamDecl) *ast.CompoundStmt {
	p.pushScope()
	defer p.popScope()
	for _, prm := range params {
		if prm.Decl != nil {
			if id := prm.Decl.DeclName(); id != nil {
				p.declare(id.Name(p.f), nameOrdinary)
			}
		}
	}
	return p.parseCompound(false)
}
