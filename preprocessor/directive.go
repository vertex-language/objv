package preprocessor

import (
	"strings"

	"github.com/vertex-language/objv/token"
)

// rejected are the directives objv does not implement, and why. The rule is
// the language's own (§1.4 of the grammar): an extension is in if it is in
// routine use in Objective-C code or in the Cocoa headers, and out otherwise.
// #import is the clearest case on the other side of that line — it is how
// every Objective-C file begins — and it is implemented rather than listed
// here.
var rejected = map[string]string{
	"assert":   "removed from GNU C itself; no replacement",
	"unassert": "removed from GNU C itself; no replacement",
	"ident":    "no effect on any target objv emits for",
	"sccs":     "no effect on any target objv emits for",
}

// directive processes one directive line. The '#' has been consumed; line
// holds the rest of the logical line, without the terminator.
func (p *Preprocessor) directive(r *reader, hash Token, line []Token) {
	at := hash.Site()

	// The null directive. It does not invalidate the multiple-include
	// optimization, matching gcc, clang and MSVC.
	if len(line) == 0 {
		return
	}

	name := line[0]
	rest := line[1:]
	word := ""
	if name.Kind == token.IDENT || name.Kind.IsKeyword() {
		word = name.Text()
	}

	// Inside a skipped group only the conditional directives are recognized;
	// everything else, including syntactically invalid directives, is skipped.
	switch word {
	case "if", "ifdef", "ifndef", "elif", "else", "endif":
	default:
		if r.skipping() {
			return
		}
	}

	// Any directive other than an opening conditional ends the window in which
	// an include guard can start.
	switch word {
	case "if", "ifdef", "ifndef":
	default:
		r.miValid = false
	}

	switch word {
	case "define":
		p.doDefine(r, rest, at)
	case "undef":
		p.doUndef(rest, at)
	case "include":
		p.doInclude(r, incInclude, rest, at)
	case "include_next":
		p.doInclude(r, incIncludeNext, rest, at)
	case "import":
		p.doInclude(r, incImport, rest, at)
	case "if":
		r.beginIf(p, "#if", at, func() bool { return p.Eval(r, rest, at) })
		r.noteGuardIf(p, rest)
	case "ifdef", "ifndef":
		p.doIfdef(r, word == "ifndef", rest, at)
	case "elif":
		r.doElif(p, at, func() bool { return p.Eval(r, rest, at) })
	case "else":
		r.doElse(p, at)
		p.expectEnd(rest, "#else")
		if c := r.topCond(); c != nil {
			c.guard = "" // an #else means the file is not simply guarded
		}
	case "endif":
		r.doEndif(p, at)
		p.expectEnd(rest, "#endif")
	case "line":
		p.doLine(rest, at)
	case "error":
		p.errorf(at, "#error %s", spell(rest))
	case "warning":
		// The directive asks for a warning, so it gets one. It is C23's
		// (WG14 N2686) and clang's long before that, which is the standard
		// every Objective-C header was written against. Named, so the
		// once-per-header rule applies in system headers: <sys/cdefs.h>
		// fires this from its unknown-compiler branch on every translation
		// unit that reaches it.
		p.warn("warning-directive", at, "#warning %s", spell(rest))
	case "pragma":
		p.doPragma(r, rest, at)
	default:
		if why, ok := rejected[word]; ok {
			p.errorf(at, "#%s is not implemented", word)
			p.note(at, why)
			return
		}
		p.errorf(name.Site(), "invalid preprocessing directive #%s", name.Text())
	}
}

// doDefine parses §6.10.3's two forms. The distinction between object-like and
// function-like is whether '(' is *adjacent* to the name: `#define M (x)`
// defines M as `(x)`, not as a macro of one parameter.
func (p *Preprocessor) doDefine(r *reader, line []Token, at Site) {
	if len(line) == 0 {
		p.errorf(at, "#define with no macro name")
		return
	}
	name := line[0]
	if !name.IsName() {
		p.errorf(name.Site(), "macro name must be an identifier")
		return
	}
	if Reserved(name.Text()) && !p.internal {
		p.errorf(name.Site(), "%q may not be redefined", name.Text())
		return
	}

	m := &Macro{Name: name.Text(), ObjLike: true, Def: name.Site()}
	body := line[1:]

	if len(body) > 0 && body[0].Kind == token.LPAREN && !body[0].Spaced() {
		var ok bool
		m.ObjLike = false
		m.Params, m.Variadic, body, ok = p.params(body[1:], at)
		if !ok {
			return
		}
	}
	m.Body = append([]Token(nil), body...)
	if !p.checkBody(m, at) {
		return
	}

	if prev := p.macros.Lookup(m.Name); prev != nil && !SameDefinition(prev, m) {
		p.warn("macro-redefined", name.Site(), "%q redefined", m.Name)
		if prev.Def.Valid() {
			p.note(prev.Def, "previous definition is here")
		}
	}
	p.macros.Define(m)
}

func (p *Preprocessor) params(line []Token, at Site) (names []string, variadic bool, body []Token, ok bool) {
	seen := map[string]bool{}
	for i := 0; i < len(line); i++ {
		t := line[i]
		switch {
		case t.Kind == token.RPAREN:
			return names, variadic, line[i+1:], true
		case t.Kind == token.ELLIPSIS:
			variadic = true
			if i+1 >= len(line) || line[i+1].Kind != token.RPAREN {
				p.errorf(t.Site(), "expected ')' after '...'")
				return nil, false, nil, false
			}
			return names, true, line[i+2:], true
		case t.IsName():
			n := t.Text()
			if n == "__VA_ARGS__" {
				p.errorf(t.Site(), "__VA_ARGS__ is not a valid parameter name")
				return nil, false, nil, false
			}
			if seen[n] {
				p.errorf(t.Site(), "duplicate macro parameter %q", n)
				return nil, false, nil, false
			}
			seen[n] = true
			names = append(names, n)
			if i+1 < len(line) && line[i+1].Kind == token.COMMA {
				i++
			}
		default:
			p.errorf(t.Site(), "expected a parameter name, found %q", t.Text())
			return nil, false, nil, false
		}
	}
	p.errorf(at, "missing ')' in macro parameter list")
	return nil, false, nil, false
}

// checkBody enforces the constraints §6.10.3.2p1 and §6.10.3.3p1 put on a
// replacement list.
func (p *Preprocessor) checkBody(m *Macro, at Site) bool {
	b := m.Body
	if len(b) > 0 && b[0].Kind == token.HASHHASH {
		p.errorf(b[0].Site(), "'##' may not appear at the start of a replacement list")
		return false
	}
	if len(b) > 0 && b[len(b)-1].Kind == token.HASHHASH {
		p.errorf(b[len(b)-1].Site(), "'##' may not appear at the end of a replacement list")
		return false
	}
	for i, t := range b {
		// A '#' must be followed by a parameter — except in the Objective-C
		// spelling of a stringized argument, `@#x`, where the '@' makes the
		// result a string object rather than a character array. The '#' is
		// still the operator and still takes a parameter; it is the token
		// before it that differs, so nothing here changes.
		if t.Kind == token.HASH && !m.ObjLike {
			if i+1 >= len(b) || m.Param(b[i+1].Text()) < 0 {
				p.errorf(t.Site(), "'#' is not followed by a macro parameter")
				return false
			}
		}
		if t.Is("__VA_ARGS__") && !m.Variadic {
			p.errorf(t.Site(), "__VA_ARGS__ may only appear in a variadic macro")
			return false
		}
	}
	return true
}

func (p *Preprocessor) doUndef(line []Token, at Site) {
	if len(line) == 0 {
		p.errorf(at, "#undef with no macro name")
		return
	}
	name := line[0]
	if !name.IsName() {
		p.errorf(name.Site(), "macro name must be an identifier")
		return
	}
	if Reserved(name.Text()) {
		p.errorf(name.Site(), "%q may not be undefined", name.Text())
		return
	}
	p.macros.Undef(name.Text())
	p.expectEnd(line[1:], "#undef")
}

func (p *Preprocessor) doIfdef(r *reader, negate bool, line []Token, at Site) {
	word := "#ifdef"
	if negate {
		word = "#ifndef"
	}
	if r.skipping() {
		r.pushCond(cond{site: at, directive: word})
		return
	}
	if len(line) == 0 || !line[0].IsName() {
		p.errorf(at, "%s with no macro name", word)
		r.pushCond(cond{site: at, directive: word})
		return
	}
	name := line[0].Text()
	// An operator the preprocessor answers for itself is defined, the same
	// as it is to `defined`. See operators.go.
	v := p.macros.Defined(name) || builtinPPMacro(name)
	if negate {
		v = !v
	}
	p.expectEnd(line[1:], word)
	c := cond{site: at, directive: word, taken: v, active: v}
	// #ifndef GUARD at the top of a file, with nothing before it, is the
	// include-guard idiom.
	if negate && r.miValid && len(r.conds) == 0 && !r.sawToken {
		c.guard = name
	}
	r.pushCond(c)
}

// noteGuardIf recognizes the other guard spelling: #if !defined FOO.
func (r *reader) noteGuardIf(p *Preprocessor, line []Token) {
	c := r.topCond()
	if c == nil || !r.miValid || len(r.conds) != 1 || r.sawToken {
		return
	}
	if len(line) >= 2 && line[0].Kind == token.NOT && line[1].Is("defined") {
		for _, t := range line[2:] {
			if t.IsName() {
				c.guard = t.Text()
				return
			}
		}
	}
}

// doLine implements §6.10.4. It changes what __LINE__ and __FILE__ report; it
// does not move any span, because a diagnostic must underline what the user
// actually typed.
func (p *Preprocessor) doLine(line []Token, at Site) {
	line = p.expandClosed(line)
	if len(line) == 0 || line[0].Kind != token.INT_LIT {
		p.errorf(at, "#line requires a digit sequence")
		return
	}
	n := 0
	for _, c := range line[0].Text() {
		if c < '0' || c > '9' {
			p.errorf(line[0].Site(), "#line requires a digit sequence")
			return
		}
		n = n*10 + int(c-'0')
	}
	p.lineDelta = n - p.physicalLine(at)
	if len(line) > 1 {
		if line[1].Kind != token.STRING_LIT {
			p.errorf(line[1].Site(), "#line filename must be a string literal")
			return
		}
		p.fileName = strings.Trim(line[1].Text(), `"`)
	}
}

// doPragma honours what it must and passes the rest through. §6.10.6 makes an
// unrecognized pragma not an error, and phase 7 is where the ones objv acts on
// are read: `#pragma clang assume_nonnull begin` is a nullability region the
// analyzer opens, and it arrives there as tokens.
//
// `#pragma once` is honoured silently. In C it is worth a note — the ISO
// spelling is an include guard, and inferring one costs nothing. In
// Objective-C it is not: #import already states the same conclusion outright,
// the language has no ISO spelling to prefer, and a header that writes the
// pragma is doing what the platform's own headers do.
func (p *Preprocessor) doPragma(r *reader, line []Token, at Site) {
	p.out = append(p.out, p.pragma(r, line, at)...)
}

// pragma returns the tokens a pragma line contributes to the output: none for
// the ones phase 4 acts on itself, and the line as written for the rest.
// _Pragma routes through here too, so the two spellings are interchangeable
// rather than merely similar.
func (p *Preprocessor) pragma(r *reader, line []Token, at Site) []Token {
	if len(line) > 0 && line[0].Is("once") {
		if r != nil {
			r.once = true
			// Immediately, not when the file finishes: the pragma has to
			// stop a re-entrant include of this same file, which happens
			// while this run is still on the stack.
			if r.cache != nil {
				r.cache.once = true
			}
		}
		return nil
	}

	// Everything else survives into the output for phase 7 to see.
	//
	// The directive walk consumed both the '#' and the word 'pragma' before
	// dispatching here, so both are re-minted — or the output is not a pragma
	// line at all: a bare '#' followed by operands glues onto the preceding
	// printed line and re-enters as garbage in declaration position. The
	// minted '#' opens a logical line, exactly as it did in the source; the
	// operand tokens keep their file spans and flags.
	hash := p.gen.Mint(token.HASH, "#")
	hash.Flags = token.FlagNLBefore
	word := p.gen.Mint(token.IDENT, "pragma")
	word.Flags = token.FlagAdjacent
	return append([]Token{hash, word}, line...)
}

// doPragmaOperator implements §6.10.9's _Pragma, which is the spelling a macro
// has to use: a macro cannot expand to a directive, so a macro that must open
// a pragma region expands to this operator instead and phase 4 turns it back
// into one.
//
// It is not a corner of the language here. NS_ASSUME_NONNULL_BEGIN is
//
//	#define NS_ASSUME_NONNULL_BEGIN _Pragma("clang assume_nonnull begin")
//
// and it stands at the top of essentially every header in a modern SDK, so a
// preprocessor without _Pragma cannot read Foundation.
//
// The string is destringized as the standard says — L prefix dropped, \" and
// \\ unescaped — and the result is scanned as if it were the rest of a
// `#pragma` line. The tokens then take the same path a written pragma takes,
// which is what makes the two spellings interchangeable rather than merely
// similar.
func (p *Preprocessor) pragmaOperator(s *stream, name Token) ([]Token, bool) {
	lp, ok := s.peek(1)
	if !ok || lp.Kind != token.LPAREN {
		return nil, false
	}
	arg, ok := p.pragmaArgument(s, name)
	if !ok {
		return nil, true // reported; the operator is consumed either way
	}
	text, ok := destringize(arg.Text())
	if !ok {
		p.errorf(arg.Site(), "_Pragma requires a string literal")
		return nil, true
	}

	f := token.NewFile("<_Pragma>", []byte(text+"\n"))
	toks, diags := scanPP(f)
	org := &Origin{File: f, System: originSystem(name)}
	for _, d := range diags {
		p.fromScan(org, d)
	}
	line := trimEOF(p.wrap(toks, org))
	for i := range line {
		// The pragma came from a macro, so a diagnostic about it must point
		// at the invocation the user wrote and not into <_Pragma>.
		line[i].Exp = &Expansion{Macro: "_Pragma", Use: name.Site(), Outer: name.Exp}

		// <_Pragma> is a file, so its first token opened a logical line
		// there. It does not open one here: these tokens follow the '#' and
		// the word 'pragma' on the line pragma() is building, and a
		// FlagNLBefore among them would print as `#pragma` alone with its
		// operands on the next line — which re-enters as a null directive
		// and a stray identifier.
		line[i].Flags &^= token.FlagNLBefore
	}
	if len(line) > 0 {
		line[0].Flags &^= token.FlagAdjacent // a space after 'pragma'
	}
	return p.pragma(p.topReader(), line, name.Site()), true
}

// pragmaArgument consumes `( "…" )` from the stream, with the operand macro-
// expanded: _Pragma(FOO) is how a header parameterizes one, and clang expands
// there.
func (p *Preprocessor) pragmaArgument(s *stream, name Token) (Token, bool) {
	s.next() // _Pragma
	s.next() // '('
	var arg []Token
	depth := 0
	for {
		t, ok := s.next()
		if !ok {
			p.errorf(name.Site(), "unterminated _Pragma operand")
			return Token{}, false
		}
		if t.Kind == token.LPAREN {
			depth++
		}
		if t.Kind == token.RPAREN {
			if depth == 0 {
				break
			}
			depth--
		}
		arg = append(arg, t)
	}
	if len(arg) == 1 && arg[0].Kind == token.STRING_LIT {
		return arg[0], true
	}
	arg = p.expandClosed(arg)
	if len(arg) == 1 && arg[0].Kind == token.STRING_LIT {
		return arg[0], true
	}
	site := name.Site()
	if len(arg) > 0 {
		site = arg[0].Site()
	}
	p.errorf(site, "_Pragma requires a string literal")
	return Token{}, false
}

// destringize reverses §6.10.9p1's transformation: drop an L prefix and the
// quotes, then replace \" with " and \\ with \. Every other backslash is kept
// as written, which is what makes `_Pragma("message(\"x\")")` work and leaves
// a pragma containing a path alone.
func destringize(s string) (string, bool) {
	s = strings.TrimPrefix(s, "L")
	if len(s) < 2 || s[0] != '"' || s[len(s)-1] != '"' {
		return "", false
	}
	s = s[1 : len(s)-1]
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) && (s[i+1] == '"' || s[i+1] == '\\') {
			i++
		}
		b.WriteByte(s[i])
	}
	return b.String(), true
}

func originSystem(t Token) bool {
	s := t.Site()
	return s.Origin != nil && s.Origin.System
}

func (p *Preprocessor) expectEnd(rest []Token, what string) {
	if len(rest) > 0 {
		p.warn("extra-tokens", rest[0].Site(), "extra tokens at end of %s directive", what)
	}
}

func spell(ts []Token) string {
	var b strings.Builder
	for i, t := range ts {
		if i > 0 && t.Spaced() {
			b.WriteByte(' ')
		}
		b.WriteString(t.Text())
	}
	return b.String()
}
