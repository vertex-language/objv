// Package token defines the lexical vocabulary of Objective-C over a
// C11 substrate (ISO/IEC 9899:2011) and the per-translation-unit
// position space every span in the front end resolves through.
//
// The vocabulary is the one docs/objc_grammar.md §2 enumerates: the 44
// C11 keywords, the Objective-C and extension keywords, the
// punctuators, the constants, and the closed set of @-directives.
//
// Invariants:
//
//  1. Nothing below the parser interprets. Tokens carry no text;
//     literals arrive undecoded and resolve through the File that
//     produced them.
//  2. No cross-file address space. A Pos is per-File.
//  3. Every span is non-empty (End > Pos), including ILLEGAL. The
//     scanner's EOF token is the one zero-width exception.
package token

// Pos is a compact position within one File: byte offset into the
// translated text, plus one, so the zero value NoPos is invalid.
type Pos int32

// NoPos is the invalid position. Fields like a delimiter that was
// never written hold NoPos.
const NoPos Pos = 0

func (p Pos) IsValid() bool { return p > NoPos }

// Flags carry lexical facts the parser ignores but diagnostics and
// formatters need.
type Flags uint8

const (
	// FlagAdjacent: no whitespace or comment separates this token
	// from the previous one.
	FlagAdjacent Flags = 1 << iota
	// FlagNLBefore: a line terminator appeared before this token.
	FlagNLBefore
	// FlagDigraph: this punctuator was spelled as a digraph
	// (<: :> <% %> %: %:%:); Kind holds the canonical punctuator.
	FlagDigraph
	// FlagSpacedAt: whitespace separated the @ from the keyword or
	// string that completes this token (§2.5's tolerated `@ interface`).
	// Only a directive or an OBJC_STRING_LIT can carry it.
	FlagSpacedAt
)

func (f Flags) Has(g Flags) bool { return f&g != 0 }

// Token is a kind and a span. It holds no text: spelling resolves
// through the File via Slice (translated) or Raw (as typed).
type Token struct {
	Kind  Kind
	Flags Flags
	Pos   Pos // inclusive
	End   Pos // exclusive
}

// SplitAngle splits a token whose first character is a > into that >
// and the remainder: >> into > and >, >>= into > and >=, >= into > and
// =. It reports whether k was one of those three.
//
// It exists for the one place lightweight generics (§5.5) collide with
// maximal munch. `NSArray<id<NSCopying>>` closes two angle-bracket
// lists with what the scanner must read as one SHR token, because
// nothing below the parser knows an angle-bracket list is open. The
// parser knows, and splits the token when it needs the first > alone;
// the halves keep exact spans, so a diagnostic still underlines the
// right character.
func (t Token) SplitAngle() (lead, rest Token, ok bool) {
	switch t.Kind {
	case SHR:
		rest.Kind = GTR
	case SHR_ASSIGN:
		rest.Kind = GEQ
	case GEQ:
		rest.Kind = ASSIGN
	default:
		return t, Token{}, false
	}
	lead = Token{Kind: GTR, Flags: t.Flags, Pos: t.Pos, End: t.Pos + 1}
	rest.Flags = FlagAdjacent
	rest.Pos, rest.End = t.Pos+1, t.End
	return lead, rest, true
}
