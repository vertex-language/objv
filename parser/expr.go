package parser

import (
	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/token"
)

// Expression: AssignmentExpression {, AssignmentExpression} (§6.7).
func (p *parser) parseExpr() ast.Expr {
	x := p.parseAssign()
	for p.at(token.COMMA) {
		op := p.pos()
		p.next()
		y := p.parseAssign()
		x = &ast.BinaryExpr{Span: ast.Span{Lo: x.Pos(), Hi: y.End()},
			X: x, OpPos: op, Op: token.COMMA, Y: y}
	}
	return x
}

// parseAssign parses a conditional expression and, if an assignment operator
// follows, builds an AssignExpr. The grammar constrains the left operand to a
// unary expression; that is a check on the finished tree, not a parsing
// decision — the same policy ConstantExpression gets.
func (p *parser) parseAssign() ast.Expr {
	x := p.parseCond()
	if isAssignOp(p.kind()) {
		op, opPos := p.kind(), p.pos()
		p.next()
		rhs := p.parseAssign() // right-associative
		return &ast.AssignExpr{Span: ast.Span{Lo: x.Pos(), Hi: rhs.End()},
			Lhs: x, OpPos: opPos, Op: op, Rhs: rhs}
	}
	return x
}

func isAssignOp(k token.Kind) bool {
	switch k {
	case token.ASSIGN, token.MUL_ASSIGN, token.QUO_ASSIGN, token.REM_ASSIGN,
		token.ADD_ASSIGN, token.SUB_ASSIGN, token.SHL_ASSIGN, token.SHR_ASSIGN,
		token.AND_ASSIGN, token.XOR_ASSIGN, token.OR_ASSIGN:
		return true
	}
	return false
}

// parseCond: LogicalOrExpression [? [Expression] : ConditionalExpression].
// ConstantExpression is this production; constant-ness is a check.
//
// The middle operand is optional, which is GCC's extension and is everywhere
// in Objective-C: `name ?: @"untitled"` reads the variable once and yields it
// when it is not nil. Every compiler that builds Cocoa accepts it.
func (p *parser) parseCond() ast.Expr {
	x := p.parseBinary(2) // 2 is ||'s level; COMMA (1) never binds here
	if !p.at(token.QUESTION) {
		return x
	}
	c := &ast.CondExpr{Cond: x, Question: p.pos()}
	p.next()
	if !p.at(token.COLON) {
		c.Then = p.parseExpr()
	}
	c.Colon = p.expect(token.COLON)
	c.Else = p.parseCond() // right-associative
	c.Span = ast.Span{Lo: x.Pos(), Hi: c.Else.End()}
	return c
}

// parseBinary is precedence climbing over §6.6's collapsed tower: one
// BinaryExpr shape, precedence from token.Precedence.
func (p *parser) parseBinary(minPrec int) ast.Expr {
	x := p.parseCastExpr()
	for {
		op := p.kind()
		prec := op.Precedence()
		if prec < minPrec {
			return x
		}
		opPos := p.pos()
		p.next()
		y := p.parseBinary(prec + 1) // all binary levels left-associate
		x = &ast.BinaryExpr{Span: ast.Span{Lo: x.Pos(), Hi: y.End()},
			X: x, OpPos: opPos, Op: op, Y: y}
	}
}

// parseCastExpr settles cast vs. parenthesized expression with the name
// table: `(T) - x` is a cast iff T is a type name. A `( type )` followed by
// `{` is a compound literal, which is postfix.
//
// §6.5's bridge casts are the same production with a keyword in front of the
// type, and the keyword makes the decision for free — nothing else may
// follow a '(' there.
func (p *parser) parseCastExpr() ast.Expr {
	p.depth++
	defer func() { p.depth-- }()
	lo := p.pos()
	if p.tooDeep() {
		return &ast.BadExpr{Span: p.span(lo)}
	}

	if p.at(token.LPAREN) && (p.isTypeNameStartAt(1) || isBridgeKeyword(p.peekTok(1).Kind)) {
		lp := p.pos()
		p.next()
		var bridge token.Pos
		var op token.Kind
		if isBridgeKeyword(p.kind()) {
			bridge, op = p.pos(), p.kind()
			p.next()
		}
		tn := p.parseTypeName()
		rp := p.expect(token.RPAREN)
		if p.at(token.LBRACE) && !bridge.IsValid() {
			cl := &ast.CompoundLit{Lparen: lp, Type: tn, Rparen: rp,
				Init: p.parseInitList()}
			cl.Span = p.span(lo)
			return p.parsePostfixSuffixes(cl)
		}
		x := p.parseCastExpr()
		return &ast.CastExpr{Span: p.span(lo), Lparen: lp,
			Bridge: bridge, Op: op, Type: tn, Rparen: rp, X: x}
	}
	return p.parseUnary()
}

func isBridgeKeyword(k token.Kind) bool {
	switch k {
	case token.BRIDGE, token.BRIDGE_RETAINED, token.BRIDGE_TRANSFER:
		return true
	}
	return false
}

func (p *parser) parseUnary() ast.Expr {
	lo := p.pos()
	switch p.kind() {
	case token.INC, token.DEC:
		op, opPos := p.kind(), p.pos()
		p.next()
		x := p.parseUnary()
		return &ast.UnaryExpr{Span: p.span(lo), OpPos: opPos, Op: op, X: x}

	case token.AND, token.MUL, token.ADD, token.SUB, token.TILDE, token.NOT:
		op, opPos := p.kind(), p.pos()
		p.next()
		x := p.parseCastExpr()
		return &ast.UnaryExpr{Span: p.span(lo), OpPos: opPos, Op: op, X: x}

	case token.SIZEOF:
		s := &ast.SizeofExpr{Sizeof: p.pos()}
		p.next()
		// sizeof ( TypeName ) iff the token after '(' opens a type; but a
		// brace after the ')' means the parenthesized type was a compound
		// literal's, and the whole thing is the operand.
		if p.at(token.LPAREN) && p.isTypeNameStartAt(1) {
			lp := p.pos()
			p.next()
			tn := p.parseTypeName()
			rp := p.expect(token.RPAREN)
			if p.at(token.LBRACE) {
				cl := &ast.CompoundLit{Lparen: lp, Type: tn, Rparen: rp,
					Init: p.parseInitList()}
				cl.Span = ast.Span{Lo: lp, Hi: p.prevEnd()}
				s.X = p.parsePostfixSuffixes(cl)
			} else {
				s.Lparen, s.Type, s.Rparen = lp, tn, rp
			}
		} else {
			s.X = p.parseUnary()
		}
		s.Span = p.span(lo)
		return s

	case token.ALIGNOF:
		// _Alignof takes a parenthesized type name; the __alignof spelling
		// §6.5 also allows over an expression resolves to the same kind, so
		// the operand decides which form this is.
		a := &ast.AlignofExpr{Alignof: p.pos()}
		p.next()
		if p.at(token.LPAREN) && p.isTypeNameStartAt(1) {
			a.Lparen = p.pos()
			p.next()
			a.Type = p.parseTypeName()
			a.Rparen = p.expect(token.RPAREN)
		} else {
			a.X = p.parseUnary()
		}
		a.Span = p.span(lo)
		return a
	}
	return p.parsePostfixSuffixes(p.parsePrimary())
}

func (p *parser) parsePostfixSuffixes(x ast.Expr) ast.Expr {
	for {
		switch p.kind() {
		case token.LBRACK:
			// A '[' after an expression subscripts it. A message send opens
			// with one instead, and parsePrimary has already taken those:
			// which construct this is depends on where the bracket is, not
			// on what is inside it.
			ix := &ast.IndexExpr{X: x, Lbrack: p.pos()}
			p.next()
			ix.Index = p.parseExpr()
			ix.Rbrack = p.expect(token.RBRACK)
			ix.Span = ast.Span{Lo: x.Pos(), Hi: p.prevEnd()}
			x = ix

		case token.LPAREN:
			c := &ast.CallExpr{Fun: x, Lparen: p.pos()}
			p.next()
			if !p.at(token.RPAREN) {
				for {
					start := p.i
					c.Args = append(c.Args, p.parseAssign())
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
			c.Rparen = p.expect(token.RPAREN)
			c.Span = ast.Span{Lo: x.Pos(), Hi: p.prevEnd()}
			x = c

		case token.PERIOD, token.ARROW:
			s := &ast.MemberExpr{X: x, OpPos: p.pos(), Op: p.kind()}
			p.next()
			// Property dot syntax reaches a name that is a selector, and a
			// selector may be spelled with a keyword: `view.class` and
			// `obj.default` are property accesses.
			s.Sel = p.expectName()
			s.Span = ast.Span{Lo: x.Pos(), Hi: p.prevEnd()}
			x = s

		case token.INC, token.DEC:
			x = &ast.IncDecExpr{Span: ast.Span{Lo: x.Pos(), Hi: p.tok().End},
				X: x, OpPos: p.pos(), Op: p.kind()}
			p.next()

		default:
			return x
		}
	}
}

func (p *parser) parsePrimary() ast.Expr {
	lo := p.pos()
	switch p.kind() {
	case token.INT_LIT, token.FLOAT_LIT, token.CHAR_LIT, token.BOOL_LIT:
		t := p.tok()
		p.next()
		return &ast.BasicLit{Span: ast.Span{Lo: t.Pos, Hi: t.End}, Kind: t.Kind}

	case token.STRING_LIT, token.OBJC_STRING_LIT:
		return p.parseStringRun()

	case token.LPAREN:
		lp := p.pos()
		p.next()
		if p.at(token.LBRACE) {
			// §6.1's statement expression. A brace cannot open an
			// expression, so the two forms are told apart by one token.
			body := p.parseCompound(true)
			rp := p.expect(token.RPAREN)
			return &ast.StmtExpr{Span: p.span(lo), Lparen: lp, Body: body, Rparen: rp}
		}
		x := p.parseExpr()
		rp := p.expect(token.RPAREN)
		return &ast.ParenExpr{Span: p.span(lo), Lparen: lp, X: x, Rparen: rp}

	case token.GENERIC:
		return p.parseGeneric(lo)

	case token.LBRACK:
		return p.parseMessage(lo)

	case token.XOR:
		return p.parseBlockLit(lo)

	case token.AT:
		return p.parseAtExpr(lo)

	case token.AT_SELECTOR:
		return p.parseSelectorExpr(lo)

	case token.AT_PROTOCOL:
		return p.parseProtocolExpr(lo)

	case token.AT_ENCODE:
		return p.parseEncodeExpr(lo)

	case token.AT_AVAILABLE, token.BUILTIN_AVAILABLE:
		return p.parseAvailability(lo)

	case token.IDENT:
		// A class name with type arguments is a receiver, and only a
		// receiver: `NSArray<NSString *>` is not an expression anywhere
		// else, so it is read here only when a message send is what
		// follows. parseMessage handles it; this is the ordinary case.
		return p.ident()
	}
	p.errHere("expected expression")
	return &ast.BadExpr{Span: p.span(lo)}
}

// parseStringRun collects one §6.1 StringLiteralSequence: one node, one span
// per piece, prefixes and any @ included.
//
// A sequence that begins with @"…" denotes a string object, and §6.1 lets its
// continuations be written either way. One that begins plain may not later
// acquire an @ — a piece with one after a plain start is where the sequence
// ends, and the @ opens whatever comes next.
func (p *parser) parseStringRun() *ast.StringLit {
	lo := p.pos()
	s := &ast.StringLit{Object: p.at(token.OBJC_STRING_LIT)}
	for {
		switch {
		case p.at(token.STRING_LIT):
		case p.at(token.OBJC_STRING_LIT) && s.Object:
		default:
			s.Span = p.span(lo)
			return s
		}
		t := p.tok()
		s.Segs = append(s.Segs, ast.Span{Lo: t.Pos, Hi: t.End})
		p.next()
	}
}

func (p *parser) parseGeneric(lo token.Pos) ast.Expr {
	g := &ast.GenericExpr{Generic: p.pos()}
	p.next()
	g.Lparen = p.expect(token.LPAREN)
	g.Ctrl = p.parseAssign()
	p.expect(token.COMMA)
	for {
		alo := p.pos()
		a := &ast.GenericAssoc{}
		if p.at(token.DEFAULT) {
			a.Default = p.pos()
			p.next()
		} else {
			a.Type = p.parseTypeName()
		}
		a.Colon = p.expect(token.COLON)
		a.Value = p.parseAssign()
		a.Span = p.span(alo)
		g.Assocs = append(g.Assocs, a)
		if !p.at(token.COMMA) {
			break
		}
		p.next()
	}
	g.Rparen = p.expect(token.RPAREN)
	g.Span = p.span(lo)
	return g
}
