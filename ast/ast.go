// Package ast defines the syntax tree built by objv's parser.
// It defines four primary node hierarchies: Expr, Stmt, Decl, and Declarator.
// Every node embeds a Span with stored start/end positions, resolving textual
// content through the underlying token.File.
package ast

import "github.com/vertex-language/objv/token"

// Node is the interface all tree nodes implement.
type Node interface {
	Pos() token.Pos // first byte
	End() token.Pos // one past the last byte
}

// Span is the stored extent every node embeds.
type Span struct {
	Lo token.Pos // inclusive
	Hi token.Pos // exclusive
}

func (s Span) Pos() token.Pos { return s.Lo }
func (s Span) End() token.Pos { return s.Hi }

// The four hierarchies. Marker methods are unexported, so the hierarchies
// are closed.

type Expr interface {
	Node
	exprNode()
}

type Stmt interface {
	Node
	stmtNode()
}

type Decl interface {
	Node
	declNode()
}

// Declarator is the type-syntax hierarchy. DeclName returns the declared
// identifier, or nil for an abstract declarator.
type Declarator interface {
	Node
	DeclName() *Ident
	declaratorNode()
}

// Ident represents an identifier or selector piece with its source span.
type Ident struct {
	Span
}

// Name returns the identifier's spelling from its translation unit.
func (id *Ident) Name(f *token.File) string {
	return string(f.Slice(id.Lo, id.Hi))
}

func (*Ident) exprNode() {}

// Tokens represents a balanced token sequence (e.g. for attribute arguments or inline asm).
type Tokens struct {
	Span
	List []token.Token `ast:"-"` // spans, not children
}

// Releaser is the interface through which ast releases backing storage.
type Releaser interface {
	Release()
}

// File is one translation unit's abstract syntax tree.
type File struct {
	Span
	Unit     *token.File   // the position space every span resolves through
	Decls    []Decl        // external declarations
	Comments []token.Token `ast:"-"` // retained under parser.ParseComments

	rel Releaser
}

// SetReleaser attaches the tree's backing storage.
func (f *File) SetReleaser(r Releaser) { f.rel = r }

// Release frees the tree's backing storage.
func (f *File) Release() {
	if f.rel != nil {
		r := f.rel
		f.rel = nil
		r.Release()
	}
}
