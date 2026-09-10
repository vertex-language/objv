package ast

import "github.com/vertex-language/objv/token"

// BadStmt covers a statement the parser gave up on.
type BadStmt struct {
	Span
}

// LabeledStmt is Identifier : Statement.
type LabeledStmt struct {
	Span
	Label *Ident
	Colon token.Pos
	Stmt  Stmt
}

// CaseStmt is case ConstantExpression : Statement, or default : Statement
// (Kind DEFAULT, Value nil). Placement discipline is checked later.
type CaseStmt struct {
	Span
	Keyword token.Pos
	Kind    token.Kind // CASE or DEFAULT
	Value   Expr       // nil for default
	Colon   token.Pos
	Stmt    Stmt
}

// CompoundStmt is '{' {BlockItem} '}'.
type CompoundStmt struct {
	Span
	Lbrace token.Pos
	Items  []Stmt // declarations arrive wrapped in DeclStmt
	Rbrace token.Pos
}

// DeclStmt adapts a Decl into a block item.
type DeclStmt struct {
	Span
	D Decl
}

// ExprStmt is [Expression] ; with a non-nil X; a lone semicolon is EmptyStmt.
type ExprStmt struct {
	Span
	X    Expr
	Semi token.Pos
}

// EmptyStmt keeps a stray semicolon visible.
type EmptyStmt struct {
	Span
	Semi token.Pos
}

// IfStmt covers both selection forms, the dangling else resolved during
// parsing by binding to the nearest unmatched if. ElsePos and Else are zero
// when there is no else.
type IfStmt struct {
	Span
	If      token.Pos
	Lparen  token.Pos
	Cond    Expr
	Rparen  token.Pos
	Then    Stmt
	ElsePos token.Pos // NoPos when no else
	Else    Stmt      // nil when no else
}

// SwitchStmt is switch ( Expression ) Statement.
type SwitchStmt struct {
	Span
	Switch token.Pos
	Lparen token.Pos
	Cond   Expr
	Rparen token.Pos
	Body   Stmt
}

// WhileStmt is while ( Expression ) Statement.
type WhileStmt struct {
	Span
	While  token.Pos
	Lparen token.Pos
	Cond   Expr
	Rparen token.Pos
	Body   Stmt
}

// DoStmt is do Statement while ( Expression ) ;
type DoStmt struct {
	Span
	Do     token.Pos
	Body   Stmt
	While  token.Pos
	Lparen token.Pos
	Cond   Expr
	Rparen token.Pos
	Semi   token.Pos
}

// ForStmt covers both C for forms. In the declaration form Init is a
// *GenDecl and Semi1 is NoPos: no ';' terminates the for-init clause
// separately, because a declaration already ends in one. In the expression
// form Init is an Expr or nil and Semi1 is the first semicolon.
//
// The fast-enumeration forms are ForInStmt; they share nothing with this but
// the keyword.
type ForStmt struct {
	Span
	For    token.Pos
	Lparen token.Pos
	Init   Node      // nil, Expr, or *GenDecl
	Semi1  token.Pos // NoPos in the declaration form
	Cond   Expr
	Semi2  token.Pos
	Post   Expr
	Rparen token.Pos
	Body   Stmt
}

// GotoStmt is goto Identifier ;
type GotoStmt struct {
	Span
	Goto  token.Pos
	Label *Ident
	Semi  token.Pos
}

// ContinueStmt is continue ;
type ContinueStmt struct {
	Span
	Continue token.Pos
	Semi     token.Pos
}

// BreakStmt is break ;
type BreakStmt struct {
	Span
	Break token.Pos
	Semi  token.Pos
}

// ReturnStmt is return [Expression] ;
type ReturnStmt struct {
	Span
	Return token.Pos
	Result Expr // nil for a bare return
	Semi   token.Pos
}

// AsmStmt is §7.4's assembly statement:
// AsmKeyword [TypeQualifier] ( BalancedTokenSequence ) ;
//
// The interior is tokens, not a parsed operand list, because the grammar says
// so and because it is right: what may appear between those parentheses —
// the template, the constraint strings, the clobbers, any goto labels —
// varies by implementation and by target, and a tree shape here would be a
// claim about a syntax this package does not define. The backend reads the
// tokens.
//
// Qual is the position of a qualifier written after the keyword (volatile, in
// practice) and NoPos when none was.
//
// The node is both a Stmt and a Decl: §3 lists an AsmStatement among the
// external declarations, where it is a file-scope assembly definition and may
// not name operands. It declares nothing — whatever its text defines, it
// defines to the linker — which is why it has no name field.
type AsmStmt struct {
	Span
	Keyword token.Pos
	Kind    token.Kind // ASM, always; the spelling is an alias
	Qual    token.Pos
	QualKey token.Kind // the qualifier's kind; ILLEGAL when Qual is NoPos
	Lparen  token.Pos
	Body    *Tokens
	Rparen  token.Pos
	Semi    token.Pos
}

func (*BadStmt) stmtNode()      {}
func (*LabeledStmt) stmtNode()  {}
func (*CaseStmt) stmtNode()     {}
func (*CompoundStmt) stmtNode() {}
func (*DeclStmt) stmtNode()     {}
func (*ExprStmt) stmtNode()     {}
func (*EmptyStmt) stmtNode()    {}
func (*IfStmt) stmtNode()       {}
func (*SwitchStmt) stmtNode()   {}
func (*WhileStmt) stmtNode()    {}
func (*DoStmt) stmtNode()       {}
func (*ForStmt) stmtNode()      {}
func (*GotoStmt) stmtNode()     {}
func (*ContinueStmt) stmtNode() {}
func (*BreakStmt) stmtNode()    {}
func (*ReturnStmt) stmtNode()   {}
func (*AsmStmt) stmtNode()      {}
func (*AsmStmt) declNode()      {}
