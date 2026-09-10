package ast

import "github.com/vertex-language/objv/token"

// ForInStmt is §7.1's fast enumeration, in both of its forms:
//
//	for (NSString *key in dict) { … }   // a declaration
//	for (key in dict) { … }             // an existing variable
//
// The declaration form fills Specs and Decl; the expression form fills X.
// They are one node because they are one statement — the collection is sent
// -countByEnumeratingWithState:objects:count: either way — and because the
// difference is exactly whether the loop variable is being declared here.
//
// It is not a ForStmt with an extra field. A C for has three clauses and two
// semicolons; this has none of them, and giving them nil fields would invite
// every consumer to ask whether they are there.
type ForInStmt struct {
	Span
	For    token.Pos
	Lparen token.Pos
	Specs  DeclSpecs  // nil in the expression form
	Decl   Declarator // nil in the expression form
	X      Expr       // nil in the declaration form
	In     token.Pos
	Coll   Expr
	Rparen token.Pos
	Body   Stmt
}

// TryStmt is §7.2's @try.
//
// A try requires at least one catch clause or a finally clause — a
// constraint, so a tree with neither can exist after the parser has reported
// on it.
type TryStmt struct {
	Span
	Keyword token.Pos
	Body    *CompoundStmt
	Catches []*CatchClause
	Finally *FinallyClause
}

// CatchClause is `@catch ( ParameterDeclaration ) CompoundStatement`, or
// `@catch ( ... )`, which catches everything and names nothing. Exactly one
// of Param and Ellipsis is set.
type CatchClause struct {
	Span
	Keyword  token.Pos
	Lparen   token.Pos
	Param    *ParamDecl
	Ellipsis token.Pos
	Rparen   token.Pos
	Body     *CompoundStmt
}

// FinallyClause is `@finally CompoundStatement`.
type FinallyClause struct {
	Span
	Keyword token.Pos
	Body    *CompoundStmt
}

// ThrowStmt is `@throw [Expression] ;`.
//
// X is nil for the bare form, which rethrows the exception currently being
// handled and is valid only inside a catch clause.
type ThrowStmt struct {
	Span
	Keyword token.Pos
	X       Expr
	Semi    token.Pos
}

// SyncStmt is §7.3's `@synchronized ( Expression ) CompoundStatement`. The
// expression must be an object pointer; it serves as the lock for the
// duration of the block.
type SyncStmt struct {
	Span
	Keyword token.Pos
	Lparen  token.Pos
	X       Expr
	Rparen  token.Pos
	Body    *CompoundStmt
}

// AutoreleasePoolStmt is §7.3's `@autoreleasepool CompoundStatement`.
type AutoreleasePoolStmt struct {
	Span
	Keyword token.Pos
	Body    *CompoundStmt
}

func (*ForInStmt) stmtNode()           {}
func (*TryStmt) stmtNode()             {}
func (*ThrowStmt) stmtNode()           {}
func (*SyncStmt) stmtNode()            {}
func (*AutoreleasePoolStmt) stmtNode() {}
