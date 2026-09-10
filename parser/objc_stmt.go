package parser

import (
	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/token"
)

// parseFor covers §7.1's four forms: the two C ones, and the two
// fast-enumeration ones.
//
// Which is which is settled by the `in` that follows the loop variable, and
// `in` is a contextual keyword — an identifier everywhere else, including as
// a variable name. So the C forms are parsed as far as the point where an
// `in` would be, and the statement becomes a ForInStmt when one is there.
func (p *parser) parseFor(lo token.Pos) ast.Stmt {
	forPos := p.pos()
	p.next()
	lparen := p.expect(token.LPAREN)

	// The loop's own scope: `for (NSString *s in xs)` declares s for the
	// body and for nothing after it.
	p.pushScope()
	defer p.popScope()

	if p.isDeclStartHere() {
		dlo := p.pos()
		specs := p.parseDeclSpecs(false)
		if p.at(token.SEMI) {
			// A declaration with no declarator: `for (int; …)` is nothing
			// anyone writes, but it parses, and the C path expects a
			// GenDecl.
			g := &ast.GenDecl{Specs: specs, Semi: p.pos()}
			p.next()
			g.Span = p.span(dlo)
			return p.finishCFor(lo, forPos, lparen, g, token.NoPos)
		}
		isTypedef := hasKeyword(specs, token.TYPEDEF)
		d := p.parseDeclarator(dmodeNormal)
		p.declareDeclarator(d, isTypedef)

		if p.atWord("in") {
			s := &ast.ForInStmt{For: forPos, Lparen: lparen, Specs: specs, Decl: d,
				In: p.pos()}
			p.next()
			s.Coll = p.parseExpr()
			s.Rparen = p.expect(token.RPAREN)
			s.Body = p.parseStmt()
			s.Span = p.span(lo)
			return s
		}
		return p.finishCFor(lo, forPos, lparen, p.finishGenDecl(dlo, specs, d), token.NoPos)
	}

	if p.at(token.SEMI) {
		semi := p.pos()
		p.next()
		return p.finishCFor(lo, forPos, lparen, nil, semi)
	}

	x := p.parseExpr()
	if p.atWord("in") {
		s := &ast.ForInStmt{For: forPos, Lparen: lparen, X: x, In: p.pos()}
		p.next()
		s.Coll = p.parseExpr()
		s.Rparen = p.expect(token.RPAREN)
		s.Body = p.parseStmt()
		s.Span = p.span(lo)
		return s
	}
	return p.finishCFor(lo, forPos, lparen, x, p.expect(token.SEMI))
}

// finishCFor reads the condition, the post-expression and the body of a C
// for, whose init clause the caller has already taken. In the declaration
// form the GenDecl owns its semicolon and Semi1 is NoPos.
func (p *parser) finishCFor(lo, forPos, lparen token.Pos, init ast.Node, semi1 token.Pos) ast.Stmt {
	s := &ast.ForStmt{For: forPos, Lparen: lparen, Init: init, Semi1: semi1}
	if !p.at(token.SEMI) {
		s.Cond = p.parseExpr()
	}
	s.Semi2 = p.expect(token.SEMI)
	if !p.at(token.RPAREN) {
		s.Post = p.parseExpr()
	}
	s.Rparen = p.expect(token.RPAREN)
	s.Body = p.parseStmt()
	s.Span = p.span(lo)
	return s
}

// parseTry reads §7.2's @try. A try needs at least one catch or a finally,
// which is a constraint rather than a parse: the node is built either way and
// the analyzer says so.
func (p *parser) parseTry(lo token.Pos) ast.Stmt {
	s := &ast.TryStmt{Keyword: p.pos()}
	p.next()
	s.Body = p.parseBraceBody("@try")

	for p.at(token.AT_CATCH) {
		clo := p.pos()
		c := &ast.CatchClause{Keyword: p.pos()}
		p.next()
		c.Lparen = p.expect(token.LPAREN)
		p.pushScope()
		if p.at(token.ELLIPSIS) {
			c.Ellipsis = p.pos()
			p.next()
		} else {
			c.Param = p.parseParamDecl()
			if c.Param.Decl != nil {
				if id := c.Param.Decl.DeclName(); id != nil {
					p.declare(id.Name(p.f), nameOrdinary)
				}
			}
		}
		c.Rparen = p.expect(token.RPAREN)
		c.Body = p.parseBraceBody("@catch")
		p.popScope()
		c.Span = p.span(clo)
		s.Catches = append(s.Catches, c)
	}

	if p.at(token.AT_FINALLY) {
		flo := p.pos()
		f := &ast.FinallyClause{Keyword: p.pos()}
		p.next()
		f.Body = p.parseBraceBody("@finally")
		f.Span = p.span(flo)
		s.Finally = f
	}
	s.Span = p.span(lo)
	return s
}

func (p *parser) parseThrow(lo token.Pos) ast.Stmt {
	s := &ast.ThrowStmt{Keyword: p.pos()}
	p.next()
	if !p.at(token.SEMI) {
		s.X = p.parseExpr()
	}
	s.Semi = p.expectSemi()
	s.Span = p.span(lo)
	return s
}

func (p *parser) parseSync(lo token.Pos) ast.Stmt {
	s := &ast.SyncStmt{Keyword: p.pos()}
	p.next()
	s.Lparen = p.expect(token.LPAREN)
	s.X = p.parseExpr()
	s.Rparen = p.expect(token.RPAREN)
	s.Body = p.parseBraceBody("@synchronized")
	s.Span = p.span(lo)
	return s
}

func (p *parser) parseAutoreleasePool(lo token.Pos) ast.Stmt {
	s := &ast.AutoreleasePoolStmt{Keyword: p.pos()}
	p.next()
	s.Body = p.parseBraceBody("@autoreleasepool")
	s.Span = p.span(lo)
	return s
}

// parseBraceBody reads a CompoundStatement where the grammar requires one —
// §7.2 and §7.3 take a compound statement and not a statement, so a body
// without braces is reported here rather than parsed into something the
// grammar does not have.
func (p *parser) parseBraceBody(what string) *ast.CompoundStmt {
	if !p.at(token.LBRACE) {
		p.errHere("expected '{' after " + what)
		lo := p.pos()
		return &ast.CompoundStmt{Span: p.span(lo)}
	}
	return p.parseCompound(true)
}
