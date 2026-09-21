// Package parser turns a *token.File into an *ast.File and diagnostics.
// It uses recursive descent for declarations and statements, and precedence climbing
// for expressions, disambiguating grammar constructs using a lexical scope table.
package parser

import (
	"fmt"

	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/scanner"
	"github.com/vertex-language/objv/token"
)

// Mode controls parser behavior.
type Mode uint

const (
	// ParseComments retains comment tokens on the File.
	ParseComments Mode = 1 << iota
	// SkipBodies skips function and method bodies without parsing their statements.
	SkipBodies
	// Tolerant continues parsing past the normal error recovery budget.
	Tolerant
)

// DefaultMode is zero.
const DefaultMode Mode = 0

const (
	maxResync = 100  // recovery attempts before going dead
	maxDepth  = 1000 // nesting cap: declarators, statements, types, expressions
)

// ParseFile runs the scanner itself and parses the unit. The tree is never
// nil; diagnostics from phases 1–2, scanning and parsing arrive merged and
// sorted. Input is expected to be preprocessed Objective-C.
func ParseFile(f *token.File, mode Mode) (*ast.File, []token.Diagnostic) {
	var sm scanner.Mode
	if mode&ParseComments != 0 {
		sm = scanner.ScanComments
	}
	toks, diags := scanner.Scan(f, sm)

	p := &parser{f: f, mode: mode, diags: diags, protocolNames: map[string]bool{}}
	file := &ast.File{Unit: f}
	file.SetReleaser(&arena{})

	if mode&ParseComments != 0 {
		for _, t := range toks {
			if t.Kind == token.COMMENT {
				file.Comments = append(file.Comments, t)
			} else {
				p.toks = append(p.toks, t)
			}
		}
	} else {
		p.toks = toks
	}

	p.pushScope() // file scope
	p.declarePredeclared()
	for !p.at(token.EOF) {
		start := p.i
		file.Decls = append(file.Decls, p.parseExternalDecl())
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

	lo := f.Pos(0)
	file.Span = ast.Span{Lo: lo, Hi: p.widen(lo, f.Pos(f.Size()))}
	token.SortDiagnostics(p.diags)
	return file, p.diags
}

// arena is the seam through which node storage will be batched; ast sees it
// only as a Releaser. Today it releases nothing — the promise (every node is
// invalid after Release) is the API; batching is an optimization this type
// reserves the right to add without changing any signature.
type arena struct{}

func (*arena) Release() {}

type parser struct {
	f    *token.File
	toks []token.Token
	i    int
	mode Mode

	diags   []token.Diagnostic
	quiet   bool      // reported; no token consumed since
	lastErr token.Pos // never report twice at one position
	resyncs int
	dead    bool // past the budget, not Tolerant: run to EOF silently
	depth   int

	scopes []map[string]nameKind

	protocolNames map[string]bool     // flat protocol namespace
	classParams   map[string][]string // generic class type parameters
	asmLabel      *ast.StringLit      // __asm("name") declarator suffix
	declAttrs     []*ast.Attr         // attribute list after declarator
	inMethodType  bool                // true inside method return/param type parens
	pack          int64               // #pragma pack alignment ceiling (0 for none)
	packStack     []int64
	classScopes   []int // scope depth stack for class/protocol scopes
}

// ---- names ----

type nameKind uint8

const (
	nameOrdinary nameKind = iota
	nameTypedef
	nameClass
	nameProtocol
	nameTypeParam
)

func (p *parser) pushScope() { p.scopes = append(p.scopes, map[string]nameKind{}) }
func (p *parser) popScope()  { p.scopes = p.scopes[:len(p.scopes)-1] }

func (p *parser) declare(name string, k nameKind) {
	if name != "" {
		p.scopes[len(p.scopes)-1][name] = k
	}
}

// pushClassScope opens a scope for class/protocol type parameters and members.
func (p *parser) pushClassScope() {
	p.pushScope()
	p.classScopes = append(p.classScopes, len(p.scopes))
}

func (p *parser) popClassScope() {
	if n := len(p.classScopes); n > 0 {
		p.classScopes = p.classScopes[:n-1]
	}
	p.popScope()
}

// inClassScope reports whether the innermost scope is a class scope.
func (p *parser) inClassScope() bool {
	n := len(p.classScopes)
	return n > 0 && p.classScopes[n-1] == len(p.scopes)
}

// declareGlobal enters a name at file scope. Protocols use their own namespace.
func (p *parser) declareGlobal(name string, k nameKind) {
	if name == "" {
		return
	}
	if k == nameProtocol {
		p.protocolNames[name] = true
		return
	}
	p.scopes[0][name] = k
}

func (p *parser) lookup(name string) nameKind {
	for i := len(p.scopes) - 1; i >= 0; i-- {
		if k, ok := p.scopes[i][name]; ok {
			return k
		}
	}
	return nameOrdinary
}

// declarePredeclared registers builtin types and Objective-C typedefs.
func (p *parser) declarePredeclared() {
	for _, n := range []string{"id", "Class", "SEL", "IMP", "BOOL", "instancetype"} {
		p.declare(n, nameTypedef)
	}
	p.declare("Protocol", nameClass)
	p.declare("__builtin_va_list", nameTypedef)
	p.declare("__int128_t", nameTypedef)
	p.declare("__uint128_t", nameTypedef)
}

func (p *parser) isTypeName(name string) bool {
	switch p.lookup(name) {
	case nameTypedef, nameClass, nameTypeParam:
		return true
	}
	return false
}

func (p *parser) isProtocolName(name string) bool { return p.protocolNames[name] }
func (p *parser) isClassName(name string) bool    { return p.lookup(name) == nameClass }

// ---- token access ----

func (p *parser) takeAsmLabel() *ast.StringLit {
	l := p.asmLabel
	p.asmLabel = nil
	return l
}

func (p *parser) takeDeclAttrs() []*ast.Attr {
	a := p.declAttrs
	p.declAttrs = nil
	return a
}

func (p *parser) tok() token.Token     { return p.toks[p.i] }
func (p *parser) kind() token.Kind     { return p.toks[p.i].Kind }
func (p *parser) at(k token.Kind) bool { return p.kind() == k }
func (p *parser) pos() token.Pos       { return p.toks[p.i].Pos }

func (p *parser) peekTok(n int) token.Token {
	if p.i+n >= len(p.toks) {
		return p.toks[len(p.toks)-1]
	}
	return p.toks[p.i+n]
}

func (p *parser) next() {
	if p.kind() != token.EOF {
		p.i++
		p.quiet = false
	}
}

func (p *parser) prevEnd() token.Pos {
	if p.i == 0 {
		return p.toks[0].Pos
	}
	return p.toks[p.i-1].End
}

// span closes a node's extent at the last consumed token; non-empty even
// when nothing was consumed.
func (p *parser) span(lo token.Pos) ast.Span {
	return ast.Span{Lo: lo, Hi: p.widen(lo, p.prevEnd())}
}

// widen returns end, or a one-column span at pos when end closes at or
// before it, so a node built from nothing still underlines something.
//
// The clamp matters at EOF, which sits at the file's one-past-the-end
// position: widening there would produce a Pos the file cannot answer for.
func (p *parser) widen(pos, end token.Pos) token.Pos {
	if end > pos {
		return end
	}
	return min(pos+1, p.f.Pos(p.f.Size()))
}

func (p *parser) name(t token.Token) string {
	return string(p.f.Slice(t.Pos, t.End))
}

// text is the spelling of the token under the cursor.
func (p *parser) text() string { return p.name(p.tok()) }

// isName reports whether a token may be used where the grammar writes
// Identifier: an identifier, or a keyword, which §4.7 admits as a selector
// piece and which a property attribute list may also hold (`copy`, `class`,
// `default`).
func isName(t token.Token) bool {
	return t.Kind == token.IDENT || t.Kind.IsKeyword()
}

// isSelector reports whether t is a valid selector piece (isName excluding __attribute__).
func isSelector(t token.Token) bool {
	return isName(t) && t.Kind != token.ATTRIBUTE
}

func (p *parser) ident() *ast.Ident {
	t := p.tok()
	p.next()
	return &ast.Ident{Span: ast.Span{Lo: t.Pos, Hi: t.End}}
}

// atWord reports whether the cursor is on an identifier with this spelling —
// the test for every contextual keyword of §2.2: `in`, `super`, `getter`,
// `oneway`, and the rest.
func (p *parser) atWord(word string) bool {
	return p.at(token.IDENT) && p.text() == word
}

// ---- diagnostics: one recoverable diagnostic, never a cascade ----

// errHere reports at the current token, then goes quiet until a token is
// consumed. It never reports twice at one position, and reports nothing once
// dead.
func (p *parser) errHere(msg string) {
	if p.dead || p.quiet {
		return
	}
	t := p.tok()
	p.quiet = true
	if t.Pos <= p.lastErr {
		return
	}
	p.lastErr = t.Pos
	p.diags = append(p.diags, token.Diagnostic{
		Pos: t.Pos, End: p.widen(t.Pos, t.End), Severity: token.Error, Message: msg,
	})
}

func (p *parser) errAt(pos token.Pos, end token.Pos, msg string) {
	if p.dead || pos <= p.lastErr {
		return
	}
	p.lastErr = pos
	p.diags = append(p.diags, token.Diagnostic{
		Pos: pos, End: p.widen(pos, end), Severity: token.Error, Message: msg,
	})
}

// warnHere reports a warning at the current token. It shares none of
// errHere's cascade machinery, deliberately: a warning is not a parse
// failure, so it does not set quiet and does not touch lastErr.
func (p *parser) warnHere(msg string) {
	if p.dead {
		return
	}
	t := p.tok()
	p.diags = append(p.diags, token.Diagnostic{
		Pos: t.Pos, End: p.widen(t.Pos, t.End), Severity: token.Warn, Message: msg,
	})
}

func (p *parser) expect(k token.Kind) token.Pos {
	if p.at(k) {
		pos := p.pos()
		p.next()
		return pos
	}
	p.errHere(fmt.Sprintf("expected '%s'", k))
	return token.NoPos
}

func (p *parser) expectSemi() token.Pos {
	if p.at(token.SEMI) {
		pos := p.pos()
		p.next()
		return pos
	}
	p.errHere("expected ';'")
	p.advanceTo(declFollow)
	if p.at(token.SEMI) {
		pos := p.pos()
		p.next()
		return pos
	}
	return token.NoPos
}

func (p *parser) expectIdent() *ast.Ident {
	if p.at(token.IDENT) {
		return p.ident()
	}
	p.errHere("expected identifier")
	return nil
}

// expectName accepts a keyword where the grammar writes Identifier or
// Selector.
func (p *parser) expectName() *ast.Ident {
	if isName(p.tok()) {
		return p.ident()
	}
	p.errHere("expected identifier")
	return nil
}

// ---- recovery ----

var (
	declFollow   = map[token.Kind]bool{token.SEMI: true, token.RBRACE: true}
	parenFollow  = map[token.Kind]bool{token.RPAREN: true, token.SEMI: true}
	brackFollow  = map[token.Kind]bool{token.RBRACK: true, token.SEMI: true}
	memberFollow = map[token.Kind]bool{
		token.SEMI: true, token.RBRACE: true, token.AT_END: true,
		token.ADD: true, token.SUB: true, token.AT_PROPERTY: true,
	}
)

// advanceTo resyncs to a follow set, stepping over balanced bracket groups.
// Past maxResync attempts it stops reporting and runs to EOF, unless
// Tolerant.
func (p *parser) advanceTo(follow map[token.Kind]bool) {
	p.resyncs++
	if p.resyncs > maxResync && p.mode&Tolerant == 0 {
		p.dead = true
	}
	if p.dead {
		p.i = len(p.toks) - 1 // the EOF token
		return
	}
	for {
		k := p.kind()
		if k == token.EOF || follow[k] {
			return
		}
		switch k {
		case token.LPAREN, token.LBRACK, token.LBRACE:
			p.skipBalanced()
		default:
			p.next()
		}
	}
}

// skipBalanced consumes an opener through its matching closer.
func (p *parser) skipBalanced() {
	depth := 0
	for {
		switch p.kind() {
		case token.LPAREN, token.LBRACK, token.LBRACE:
			depth++
		case token.RPAREN, token.RBRACK, token.RBRACE:
			depth--
		case token.EOF:
			return
		}
		p.next()
		if depth <= 0 {
			return
		}
	}
}

// tooDeep is the maxDepth guard; on breach it reports once and consumes a
// token so callers returning Bad* still make progress.
func (p *parser) tooDeep() bool {
	if p.depth <= maxDepth {
		return false
	}
	p.errHere("nesting too deep")
	if !p.at(token.EOF) {
		p.next()
	}
	return true
}

// ---- classification ----

// isTypeSpecStart: can this token open a specifier-qualifier list?
func (p *parser) isTypeSpecStart(t token.Token) bool {
	switch t.Kind {
	case token.VOID, token.CHAR, token.SHORT, token.INT, token.LONG,
		token.FLOAT, token.DOUBLE, token.SIGNED, token.UNSIGNED,
		token.BOOL, token.COMPLEX, token.AUTO_TYPE, token.INT128,
		token.FLOAT16, token.STRUCT,
		token.UNION, token.ENUM, token.ATOMIC, token.CONST, token.RESTRICT,
		token.VOLATILE, token.TYPEOF, token.ATTRIBUTE,
		// §5.6's Objective-C qualifiers open one wherever const does.
		token.KINDOF, token.STRONG, token.WEAK, token.UNSAFE_UNRETAINED,
		token.AUTORELEASING, token.NONNULL, token.NULLABLE,
		token.NULL_UNSPECIFIED, token.PTRAUTH:
		return true
	case token.IDENT:
		return p.isTypeName(p.name(t))
	}
	return false
}

// isDeclSpecStart adds the declaration-only specifiers.
func (p *parser) isDeclSpecStart(t token.Token) bool {
	switch t.Kind {
	case token.TYPEDEF, token.EXTERN, token.STATIC, token.THREAD_LOCAL,
		token.AUTO, token.REGISTER, token.INLINE, token.NORETURN,
		token.ALIGNAS, token.BLOCK:
		return true
	}
	return p.isTypeSpecStart(t)
}

// isDeclStartHere settles declaration vs. expression statement by the first
// token.
func (p *parser) isDeclStartHere() bool {
	if p.at(token.STATIC_ASSERT) {
		return true
	}
	// A label is not a declaration, whatever T means: label names are their
	// own namespace, so `T: ;` labels even when T is a class.
	if p.at(token.IDENT) && p.peekTok(1).Kind == token.COLON {
		return false
	}
	// No declarator starts with '.', so a name followed by one is an
	// expression even when the name is a type: `Config.level = 9;` is a
	// class property, reached through its class's name.
	if p.at(token.IDENT) && p.peekTok(1).Kind == token.PERIOD {
		return false
	}
	return p.isDeclSpecStart(p.tok())
}

func (p *parser) isTypeNameStartAt(n int) bool {
	return p.isTypeSpecStart(p.peekTok(n))
}
