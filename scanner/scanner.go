// Package scanner turns a *token.File into a complete token slice.
//
// The whole unit is tokenized up front. Every scan path advances at
// least one byte; malformed input yields an exact span and one
// diagnostic, never a cascade. Nothing is interpreted: literals keep
// their raw spelling, and typedef-ness — along with class names,
// protocol names, and the contextual keywords of §2.2 — is the
// parser's business.
package scanner

import (
	"bytes"
	"fmt"
	"unicode/utf8"

	"github.com/vertex-language/objv/token"
)

// Mode controls optional scanner behavior.
type Mode uint

const (
	// ScanComments keeps COMMENT tokens in the stream; without it
	// they are trivia, still reachable via token.File.Between.
	ScanComments Mode = 1 << iota

	// ScanPP scans preprocessing tokens for source files prior to phase 4.
	// Line-opening '#' is emitted as HASH, bracket balancing is disabled,
	// the first token has FlagNLBefore, and value-level literal diagnostics
	// are deferred until expansion or re-scanning.
	ScanPP

	// NoDollarIdents drops '$' from the identifier alphabet (§2.1 extension, on by default).
	NoDollarIdents
)

// Scan tokenizes f. The returned token slice always ends in an EOF token.
// Diagnostics are sorted, including merged phase 1–2 diagnostics from f.
func Scan(f *token.File, mode Mode) ([]token.Token, []token.Diagnostic) {
	s := &scanner{
		f:        f,
		text:     f.Text(),
		mode:     mode,
		dollar:   mode&NoDollarIdents == 0,
		lineOpen: true,
		nlBefore: mode&ScanPP != 0,
	}
	s.diags = append(s.diags, f.Diagnostics()...)
	for {
		s.skipTrivia()
		if s.off >= len(s.text) {
			break
		}
		s.scanToken()
	}
	s.emit(token.EOF, s.off) // the one zero-width span
	if s.mode&ScanPP == 0 {
		s.atEOF()
	}
	token.SortDiagnostics(s.diags)
	return s.toks, s.diags
}

type scanner struct {
	f      *token.File
	text   []byte
	mode   Mode
	dollar bool // $ is an identifier character
	off    int

	toks  []token.Token
	diags []token.Diagnostic

	adjacent bool // no trivia since the previous token
	nlBefore bool // line terminator since the previous token
	lineOpen bool // no token yet on this logical line
	quietTok bool // current token already reported

	hashReported bool // the once-per-file directive-line report

	brackets     []bracket
	bracketQuiet bool
}

func (s *scanner) peek(i int) byte {
	if s.off+i < len(s.text) {
		return s.text[s.off+i]
	}
	return 0
}

// ---- trivia ----

func (s *scanner) skipTrivia() {
	for s.off < len(s.text) {
		switch c := s.text[s.off]; {
		case c == ' ' || c == '\t' || c == '\v' || c == '\f':
			s.off++
			s.adjacent = false

		case c == '\n' || c == '\r':
			s.off++
			if c == '\r' && s.off < len(s.text) && s.text[s.off] == '\n' {
				s.off++
			}
			s.adjacent = false
			s.nlBefore = true
			s.lineOpen = true

		case c == '/' && s.peek(1) == '/':
			start := s.off
			for s.off < len(s.text) && s.text[s.off] != '\n' && s.text[s.off] != '\r' {
				s.off++
			}
			s.comment(start)

		case c == '/' && s.peek(1) == '*':
			start := s.off
			s.off += 2
			closed := false
			for s.off < len(s.text) {
				if s.text[s.off] == '*' && s.peek(1) == '/' {
					s.off += 2
					closed = true
					break
				}
				s.off++
			}
			if !closed {
				s.report(token.Error, start, s.off, "unterminated /* comment")
			}
			s.comment(start)

		default:
			return
		}
	}
}

func (s *scanner) comment(start int) {
	if bytes.ContainsAny(s.text[start:s.off], "\n\r") {
		s.nlBefore = true
	}
	if s.mode&ScanComments != 0 {
		s.emit(token.COMMENT, start)
	}
	s.adjacent = false
}

// ---- tokens ----

func (s *scanner) scanToken() {
	s.quietTok = false
	c := s.text[s.off]

	// Outside ScanPP, a line-opening '#' (or '%:') is skipped as trivia unless
	// it begins a #pragma (which must reach the parser for e.g. #pragma pack).
	if s.lineOpen && s.mode&ScanPP == 0 && (c == '#' || (c == '%' && s.peek(1) == ':')) &&
		!s.atPragma() {
		s.directiveLine()
		return
	}
	s.lineOpen = false

	switch {
	case c == 'u' || c == 'U' || c == 'L':
		s.scanIdentOrLiteralPrefix()
	case s.identStart(c) || c == '\\':
		s.scanIdent(s.off)
	case isDigit(c) || (c == '.' && isDigit(s.peek(1))):
		s.scanNumber()
	case c == '\'':
		s.scanChar(s.off)
	case c == '"':
		s.scanString(s.off)
	case c == '@':
		s.scanAt()
	case c >= 0x80:
		s.scanExtended()
	default:
		s.scanPunct()
	}
}

// atPragma reports whether the line-opening '#' at the cursor begins a
// #pragma. It looks ahead over horizontal whitespace only: a directive name
// is on the same logical line as its '#', and line splices were already
// removed in phase 2.
func (s *scanner) atPragma() bool {
	i := s.off + 1
	if s.text[s.off] == '%' {
		i++
	}
	for i < len(s.text) && (s.text[i] == ' ' || s.text[i] == '\t') {
		i++
	}
	const word = "pragma"
	if i+len(word) > len(s.text) || string(s.text[i:i+len(word)]) != word {
		return false
	}
	i += len(word)
	// The word has to end here, or `#pragmatic` would be one.
	return i >= len(s.text) || !isIdentPart(s.text[i])
}

func (s *scanner) directiveLine() {
	start := s.off
	for s.off < len(s.text) && s.text[s.off] != '\n' && s.text[s.off] != '\r' {
		s.off++
	}
	// Skip line markers (e.g. `# 42 "foo.m" 3` or `#line ...`) without warning.
	if !s.hashReported && !isLineMarker(s.text[start:s.off]) {
		s.hashReported = true
		s.report(token.Warn, start, s.off,
			"preprocessor directive in preprocessed input; objv scans phase 5 source here — pass the .m, or scan with ScanPP")
	}
	s.adjacent = false
}

// isLineMarker reports whether a line-opening '#' begins a line marker: the
// '#' followed by a decimal line number, and then whatever the producer chose
// to add. `#line 42 "foo.m"` is the ISO spelling of the same thing and is
// recognized too.
func isLineMarker(line []byte) bool {
	i := 0
	for i < len(line) && (line[i] == '#' || line[i] == ' ' || line[i] == '\t') {
		i++
	}
	if rest := line[i:]; len(rest) >= 4 && string(rest[:4]) == "line" {
		i += 4
		for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
			i++
		}
	}
	if i >= len(line) || line[i] < '0' || line[i] > '9' {
		return false
	}
	return true
}

// scanExtended consumes a run of non-ASCII bytes as a single ILLEGAL token (§2.1).
func (s *scanner) scanExtended() {
	start := s.off
	for s.off < len(s.text) && s.text[s.off] >= 0x80 {
		s.off++
	}
	r, _ := utf8.DecodeRune(s.text[start:s.off])
	s.other(start, fmt.Sprintf(
		"extended character %q outside a comment or literal; write it as a universal character name", r))
}

// ---- punctuators: maximal munch, digraphs collapse to canonical kinds ----

func (s *scanner) scanPunct() {
	start := s.off
	c, c1, c2 := s.text[s.off], s.peek(1), s.peek(2)
	var k token.Kind
	var fl token.Flags
	adv := 1

	switch c {
	case '[':
		k = token.LBRACK
	case ']':
		k = token.RBRACK
	case '(':
		k = token.LPAREN
	case ')':
		k = token.RPAREN
	case '{':
		k = token.LBRACE
	case '}':
		k = token.RBRACE
	case ';':
		k = token.SEMI
	case ',':
		k = token.COMMA
	case '?':
		k = token.QUESTION
	case '~':
		k = token.TILDE
	case '.':
		if c1 == '.' && c2 == '.' {
			k, adv = token.ELLIPSIS, 3
		} else {
			k = token.PERIOD // ".." is PERIOD PERIOD
		}
	case '+':
		switch c1 {
		case '+':
			k, adv = token.INC, 2
		case '=':
			k, adv = token.ADD_ASSIGN, 2
		default:
			k = token.ADD // also §4.7's MethodKind
		}
	case '-':
		switch c1 {
		case '>':
			k, adv = token.ARROW, 2
		case '-':
			k, adv = token.DEC, 2
		case '=':
			k, adv = token.SUB_ASSIGN, 2
		default:
			k = token.SUB // also §4.7's MethodKind
		}
	case '*':
		if c1 == '=' {
			k, adv = token.MUL_ASSIGN, 2
		} else {
			k = token.MUL
		}
	case '/':
		if c1 == '=' {
			k, adv = token.QUO_ASSIGN, 2
		} else {
			k = token.QUO // comments were consumed as trivia
		}
	case '!':
		if c1 == '=' {
			k, adv = token.NEQ, 2
		} else {
			k = token.NOT
		}
	case '=':
		if c1 == '=' {
			k, adv = token.EQL, 2
		} else {
			k = token.ASSIGN
		}
	case '^':
		if c1 == '=' {
			k, adv = token.XOR_ASSIGN, 2
		} else {
			k = token.XOR // also the block introducer of §5.7 and §6.9
		}
	case '&':
		switch c1 {
		case '&':
			k, adv = token.LAND, 2
		case '=':
			k, adv = token.AND_ASSIGN, 2
		default:
			k = token.AND
		}
	case '|':
		switch c1 {
		case '|':
			k, adv = token.LOR, 2
		case '=':
			k, adv = token.OR_ASSIGN, 2
		default:
			k = token.OR
		}
	case '<':
		switch {
		case c1 == '<' && c2 == '=':
			k, adv = token.SHL_ASSIGN, 3
		case c1 == '<':
			k, adv = token.SHL, 2
		case c1 == '=':
			k, adv = token.LEQ, 2
		case c1 == ':':
			k, adv, fl = token.LBRACK, 2, token.FlagDigraph
		case c1 == '%':
			k, adv, fl = token.LBRACE, 2, token.FlagDigraph
		default:
			k = token.LSS // also opens §5.5's angle-bracket lists
		}
	case '>':
		switch {
		case c1 == '>' && c2 == '=':
			k, adv = token.SHR_ASSIGN, 3
		case c1 == '>':
			k, adv = token.SHR, 2 // token.Token.SplitAngle undoes this
		case c1 == '=':
			k, adv = token.GEQ, 2
		default:
			k = token.GTR
		}
	case ':':
		if c1 == '>' {
			k, adv, fl = token.RBRACK, 2, token.FlagDigraph
		} else {
			k = token.COLON // :: is two of these; see token/kind.go
		}
	case '%':
		switch {
		case c1 == ':' && c2 == '%' && s.peek(3) == ':':
			k, adv, fl = token.HASHHASH, 4, token.FlagDigraph
		case c1 == ':':
			k, adv, fl = token.HASH, 2, token.FlagDigraph
		case c1 == '>':
			k, adv, fl = token.RBRACE, 2, token.FlagDigraph
		case c1 == '=':
			k, adv = token.REM_ASSIGN, 2
		default:
			k = token.REM
		}
	case '#':
		if c1 == '#' {
			k, adv = token.HASHHASH, 2
		} else {
			k = token.HASH
		}
	default:
		s.off++
		s.other(start, fmt.Sprintf("illegal character %q", c))
		return
	}

	s.off += adv
	// Bracket balance is a claim about preprocessed source. A header
	// that defines `{` as a macro is not unbalanced, it is a header.
	if s.mode&ScanPP == 0 {
		s.bracket(k, start)
	}
	s.emitFlags(k, start, fl)
}

// ---- the advisory bracket stack: never affects tokenization ----

type bracket struct {
	kind token.Kind
	pos  int
}

func (s *scanner) bracket(k token.Kind, pos int) {
	switch k {
	case token.LPAREN, token.LBRACK, token.LBRACE:
		s.brackets = append(s.brackets, bracket{k, pos})
	case token.RPAREN, token.RBRACK, token.RBRACE:
		if len(s.brackets) == 0 {
			s.bracketErr(pos, fmt.Sprintf("unmatched %s", k))
			return
		}
		top := s.brackets[len(s.brackets)-1]
		s.brackets = s.brackets[:len(s.brackets)-1]
		if closerFor(top.kind) != k {
			// Blame the opener, then go quiet.
			s.bracketErr(top.pos, fmt.Sprintf("unclosed %s, closed by %s", top.kind, k))
		}
	}
}

func (s *scanner) atEOF() {
	if len(s.brackets) > 0 {
		top := s.brackets[len(s.brackets)-1] // the innermost one
		s.bracketErr(top.pos, fmt.Sprintf("unclosed %s at end of file", top.kind))
	}
}

func (s *scanner) bracketErr(pos int, msg string) {
	if s.bracketQuiet {
		return
	}
	s.bracketQuiet = true
	s.report(token.Error, pos, pos+1, msg)
}

func closerFor(k token.Kind) token.Kind {
	switch k {
	case token.LPAREN:
		return token.RPAREN
	case token.LBRACK:
		return token.RBRACK
	}
	return token.RBRACE
}

// ---- emission and diagnostics ----

func (s *scanner) emit(k token.Kind, start int) { s.emitFlags(k, start, 0) }

func (s *scanner) emitFlags(k token.Kind, start int, fl token.Flags) {
	if s.adjacent {
		fl |= token.FlagAdjacent
	}
	if s.nlBefore {
		fl |= token.FlagNLBefore
	}
	s.toks = append(s.toks, token.Token{
		Kind: k, Flags: fl,
		Pos: s.f.Pos(start), End: s.f.Pos(s.off),
	})
	s.adjacent = true
	s.nlBefore = false
}

// other emits an ILLEGAL token for non-whitespace characters not matching any
// other pp-token (C11 §6.4p1). Under ScanPP errors are deferred until phase 7.
func (s *scanner) other(start int, msg string) {
	if s.mode&ScanPP == 0 {
		s.errTok(start, s.off, msg)
	}
	s.emit(token.ILLEGAL, start)
}

// errTok reports at most once per token: after the first report the
// current token goes quiet.
func (s *scanner) errTok(lo, hi int, msg string) {
	if s.quietTok {
		return
	}
	s.quietTok = true
	s.report(token.Error, lo, hi, msg)
}

// valueErr reports semantic token errors (invalid suffix, digit, etc.).
// Under ScanPP, value errors are deferred until phase 7.
func (s *scanner) valueErr(lo, hi int, msg string) {
	if s.mode&ScanPP != 0 {
		return
	}
	s.errTok(lo, hi, msg)
}

// report appends one diagnostic with a non-empty span clamped to the
// source.
func (s *scanner) report(sev token.Severity, lo, hi int, msg string) {
	n := len(s.text)
	if n == 0 {
		return
	}
	if hi <= lo {
		hi = lo + 1
	}
	if hi > n {
		hi = n
	}
	if lo >= hi {
		lo = hi - 1
	}
	s.diags = append(s.diags, token.Diagnostic{
		Pos: s.f.Pos(lo), End: s.f.Pos(hi), Severity: sev, Message: msg,
	})
}

func isDigit(c byte) bool    { return '0' <= c && c <= '9' }
func isLetter(c byte) bool   { return 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' }
func isBinDigit(c byte) bool { return c == '0' || c == '1' }

func isHexDigit(c byte) bool {
	return isDigit(c) || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F'
}
func isIdentStart(c byte) bool { return c == '_' || isLetter(c) }
func isIdentPart(c byte) bool  { return isIdentStart(c) || isDigit(c) }

// identStart and identPart add §2.1's $ extension, which NoDollarIdents
// turns off.
func (s *scanner) identStart(c byte) bool { return isIdentStart(c) || (c == '$' && s.dollar) }
func (s *scanner) identPart(c byte) bool  { return isIdentPart(c) || (c == '$' && s.dollar) }
