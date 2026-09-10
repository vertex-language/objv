package parser

import (
	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/token"
)

func (p *parser) parseStmt() ast.Stmt {
	p.depth++
	defer func() { p.depth-- }()
	lo := p.pos()
	if p.tooDeep() {
		return &ast.BadStmt{Span: p.span(lo)}
	}

	switch p.kind() {
	case token.IDENT:
		// Labels first: label names are their own namespace, so `T: ;`
		// labels even when T is a class or a typedef.
		if p.peekTok(1).Kind == token.COLON {
			l := &ast.LabeledStmt{Label: p.ident()}
			l.Colon = p.pos()
			p.next()
			l.Stmt = p.parseStmt()
			l.Span = p.span(lo)
			return l
		}
		if p.isDeclStartHere() {
			return p.declStmt(lo)
		}
		return p.parseExprStmt(lo)

	case token.CASE, token.DEFAULT:
		c := &ast.CaseStmt{Keyword: p.pos(), Kind: p.kind()}
		p.next()
		if c.Kind == token.CASE {
			c.Value = p.parseCond()
		}
		c.Colon = p.expect(token.COLON)
		c.Stmt = p.parseStmt()
		c.Span = p.span(lo)
		return c

	case token.LBRACE:
		return p.parseCompound(true)

	case token.SEMI:
		semi := p.pos()
		p.next()
		return &ast.EmptyStmt{Span: p.span(lo), Semi: semi}

	case token.IF:
		s := &ast.IfStmt{If: p.pos()}
		p.next()
		s.Lparen = p.expect(token.LPAREN)
		s.Cond = p.parseExpr()
		s.Rparen = p.expect(token.RPAREN)
		s.Then = p.parseStmt()
		// The dangling else binds to the nearest unmatched if, which this
		// call structure produces naturally.
		if p.at(token.ELSE) {
			s.ElsePos = p.pos()
			p.next()
			s.Else = p.parseStmt()
		}
		s.Span = p.span(lo)
		return s

	case token.SWITCH:
		s := &ast.SwitchStmt{Switch: p.pos()}
		p.next()
		s.Lparen = p.expect(token.LPAREN)
		s.Cond = p.parseExpr()
		s.Rparen = p.expect(token.RPAREN)
		s.Body = p.parseStmt()
		s.Span = p.span(lo)
		return s

	case token.WHILE:
		s := &ast.WhileStmt{While: p.pos()}
		p.next()
		s.Lparen = p.expect(token.LPAREN)
		s.Cond = p.parseExpr()
		s.Rparen = p.expect(token.RPAREN)
		s.Body = p.parseStmt()
		s.Span = p.span(lo)
		return s

	case token.DO:
		s := &ast.DoStmt{Do: p.pos()}
		p.next()
		s.Body = p.parseStmt()
		s.While = p.expect(token.WHILE)
		s.Lparen = p.expect(token.LPAREN)
		s.Cond = p.parseExpr()
		s.Rparen = p.expect(token.RPAREN)
		s.Semi = p.expectSemi()
		s.Span = p.span(lo)
		return s

	case token.FOR:
		return p.parseFor(lo)

	case token.GOTO:
		s := &ast.GotoStmt{Goto: p.pos()}
		p.next()
		s.Label = p.expectIdent() // target discipline is checked later
		s.Semi = p.expectSemi()
		s.Span = p.span(lo)
		return s

	case token.CONTINUE:
		s := &ast.ContinueStmt{Continue: p.pos()}
		p.next()
		s.Semi = p.expectSemi()
		s.Span = p.span(lo)
		return s

	case token.BREAK:
		s := &ast.BreakStmt{Break: p.pos()}
		p.next()
		s.Semi = p.expectSemi()
		s.Span = p.span(lo)
		return s

	case token.RETURN:
		s := &ast.ReturnStmt{Return: p.pos()}
		p.next()
		if !p.at(token.SEMI) {
			s.Result = p.parseExpr()
		}
		s.Semi = p.expectSemi()
		s.Span = p.span(lo)
		return s

	case token.ASM:
		return p.parseAsmStmt(lo)

	case token.AT_TRY:
		return p.parseTry(lo)

	case token.AT_THROW:
		return p.parseThrow(lo)

	case token.AT_SYNCHRONIZED:
		return p.parseSync(lo)

	case token.AT_AUTORELEASEPOOL:
		return p.parseAutoreleasePool(lo)

	case token.HASH:
		p.skipDirectiveLine()
		return &ast.EmptyStmt{Span: p.span(lo)}

	default:
		if p.isDeclStartHere() {
			return p.declStmt(lo)
		}
		return p.parseExprStmt(lo)
	}
}

func (p *parser) declStmt(lo token.Pos) ast.Stmt {
	d := p.parseDeclaration()
	return &ast.DeclStmt{Span: ast.Span{Lo: d.Pos(), Hi: d.End()}, D: d}
}

func (p *parser) parseExprStmt(lo token.Pos) ast.Stmt {
	x := p.parseExpr()
	semi := p.expectSemi()
	return &ast.ExprStmt{Span: p.span(lo), X: x, Semi: semi}
}

// parseCompound parses '{' {BlockItem} '}'. push is false when the caller
// already opened the scope — a function body, a method body and a block's
// body all share a scope with their parameters.
func (p *parser) parseCompound(push bool) *ast.CompoundStmt {
	lo := p.pos()
	cs := &ast.CompoundStmt{Lbrace: p.expect(token.LBRACE)}
	if push {
		p.pushScope()
		defer p.popScope()
	}
	for !p.at(token.RBRACE) && !p.at(token.EOF) {
		start := p.i
		cs.Items = append(cs.Items, p.parseStmt())
		if p.i == start { // progress check: force a resync
			p.advanceTo(declFollow)
			if p.at(token.SEMI) {
				p.next()
			}
			if p.i == start {
				p.next()
			}
		}
	}
	cs.Rbrace = p.expect(token.RBRACE)
	cs.Span = p.span(lo)
	return cs
}

// parseAsmStmt reads §7.4's assembly statement. The interior is a balanced
// token sequence, which is what the grammar makes it: what may appear between
// those parentheses varies by target, and the backend is the layer that
// knows.
func (p *parser) parseAsmStmt(lo token.Pos) ast.Stmt {
	a := &ast.AsmStmt{Keyword: p.pos(), Kind: token.ASM}
	p.next()
	switch p.kind() {
	case token.VOLATILE, token.CONST, token.RESTRICT, token.INLINE, token.GOTO:
		a.Qual, a.QualKey = p.pos(), p.kind()
		p.next()
	}
	a.Lparen = p.expect(token.LPAREN)
	a.Body = p.parseBalanced()
	a.Rparen = p.expect(token.RPAREN)
	if p.at(token.SEMI) {
		a.Semi = p.pos()
		p.next()
	}
	a.Span = p.span(lo)
	return a
}
