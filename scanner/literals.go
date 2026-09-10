package scanner

import (
	"fmt"
	"strings"

	"github.com/vertex-language/objv/token"
)

// scanIdentOrLiteralPrefix handles u, U, L: maximal munch means u8"…",
// u"…", U"…", L"…", u'…', U'…', L'…' are single literal tokens.
func (s *scanner) scanIdentOrLiteralPrefix() {
	start := s.off
	c := s.text[s.off]
	switch {
	case c == 'u' && s.peek(1) == '8' && s.peek(2) == '"':
		s.off += 2
		s.scanString(start)
	case s.peek(1) == '"':
		s.off++
		s.scanString(start)
	case s.peek(1) == '\'':
		s.off++
		s.scanChar(start)
	default:
		s.scanIdent(start)
	}
}

// scanIdent consumes an identifier, universal character names
// included (undecoded, per invariant — the span keeps the \uXXXX
// spelling). Keyword lookup is by exact spelling; typedef-ness, class
// names, and the contextual keywords of §2.2 are the parser's call.
func (s *scanner) scanIdent(start int) {
	if s.text[s.off] == '\\' && s.peek(1) != 'u' && s.peek(1) != 'U' {
		s.off++
		s.other(start, `stray '\' in program`)
		return
	}
	for s.off < len(s.text) {
		c := s.text[s.off]
		switch {
		case s.identPart(c):
			s.off++
		case c == '\\' && (s.peek(1) == 'u' || s.peek(1) == 'U'):
			s.scanUCN()
		default:
			s.emit(token.Lookup(string(s.text[start:s.off])), start)
			return
		}
	}
	s.emit(token.Lookup(string(s.text[start:s.off])), start)
}

// scanUCN consumes \uXXXX or \UXXXXXXXX starting at the backslash.
// A malformed UCN is one diagnostic; scanning continues. This is a
// formation error, not a value error: it reports in every mode.
func (s *scanner) scanUCN() {
	start := s.off
	need := 4
	if s.text[s.off+1] == 'U' {
		need = 8
	}
	s.off += 2
	got := 0
	for got < need && s.off < len(s.text) && isHexDigit(s.text[s.off]) {
		s.off++
		got++
	}
	if got < need {
		s.errTok(start, s.off, fmt.Sprintf(
			"malformed universal character name (need %d hexadecimal digits, have %d)", need, got))
	}
}

// scanNumber consumes one numeric run — §2.3's PPNumber: digits,
// identifier characters, '.', and exponent signs — then classifies it
// as a whole. Structural mistakes report once per run, never twice;
// a suffix that is not a suffix is not one of them, for the reason
// classify gives.
func (s *scanner) scanNumber() {
	start := s.off
	hex := s.text[s.off] == '0' && (s.peek(1) == 'x' || s.peek(1) == 'X')
	bin := s.text[s.off] == '0' && (s.peek(1) == 'b' || s.peek(1) == 'B')
loop:
	for s.off < len(s.text) {
		c := s.text[s.off]
		switch {
		case isDigit(c) || c == '.':
			s.off++
		case s.identStart(c):
			// §2.3's PPNumber admits an IdentifierNondigit, which is any
			// identifier character that is not a digit — the underscore
			// included. It is not a curiosity: Apple's CF_AVAILABLE(10_0,
			// 2_0) pastes its argument onto __MAC_ and needs 10_0 to be one
			// token, and a scanner that stopped at the underscore produces
			// __MAC_10 followed by a stray _0, which then fails to expand
			// as the macro it was supposed to name.
			s.off++
			sign := s.peek(0) == '+' || s.peek(0) == '-'
			if sign && ((!hex && (c == 'e' || c == 'E')) || (hex && (c == 'p' || c == 'P'))) {
				s.off++
			}
		default:
			break loop
		}
	}
	s.emit(s.classify(start, hex, bin), start)
}

// classify enforces the lexical grammar of §2.3 over the consumed run
// and picks INT_LIT or FLOAT_LIT. The literal's value is a decoding
// concern, phases above this one.
//
// Under ScanPP the reports defer (valueErr): the run is a legal
// pp-number whatever it fails to classify as — 0779 and 10.12.2 may
// live and die in a macro body or an excluded group without being
// anyone's mistake. The classification itself still happens; phase 4
// keys on it.
func (s *scanner) classify(start int, hex, bin bool) token.Kind {
	t := s.text[start:s.off]
	fail := func(msg string) { s.valueErr(start, s.off, msg) }

	// A run with two or more dots is not a constant in any base — one
	// dot is the most a decimal or hexadecimal constant has — and it is
	// the shape a version number takes. §6.10's VersionTuple is written
	// exactly this way, `@available(macOS 10.12.1, *)`, and so is the
	// availability attribute the Cocoa headers put on nearly every
	// declaration. It is a legal pp-number that no phase gives a value
	// to, so there is nothing to report here; the parser reads the
	// digits back out of the span.
	if dots(t) >= 2 {
		return token.FLOAT_LIT
	}

	i := 0
	run := func(pred func(byte) bool) int {
		n := 0
		for i < len(t) && pred(t[i]) {
			i++
			n++
		}
		return n
	}

	isFloat := false
	if bin {
		// gcc's binary constant, C23's too and spelled the same way.
		// There is no floating form: 0b1011 is an integer and nothing
		// else.
		i = 2
		if run(isBinDigit) == 0 {
			fail("binary constant requires at least one digit")
			return token.INT_LIT
		}
		if suffix := t[i:]; !validIntSuffix(suffix) {
			fail(fmt.Sprintf("invalid suffix %q on integer constant", string(suffix)))
		}
		return token.INT_LIT
	}
	if hex {
		i = 2
		n := run(isHexDigit)
		if i < len(t) && t[i] == '.' {
			i++
			isFloat = true
			n += run(isHexDigit)
		}
		if n == 0 {
			fail("hexadecimal constant requires at least one digit")
			return token.INT_LIT
		}
		if i < len(t) && (t[i] == 'p' || t[i] == 'P') {
			isFloat = true
			i++
			if i < len(t) && (t[i] == '+' || t[i] == '-') {
				i++
			}
			if run(isDigit) == 0 { // binary exponents take decimal digits
				fail("binary exponent requires decimal digits")
				return token.FLOAT_LIT
			}
		} else if isFloat {
			fail("hexadecimal floating constant requires a binary exponent")
			return token.FLOAT_LIT
		}
	} else {
		run(isDigit)
		if i < len(t) && t[i] == '.' {
			i++
			isFloat = true
			run(isDigit)
		}
		if i < len(t) && (t[i] == 'e' || t[i] == 'E') {
			i++
			isFloat = true
			if i < len(t) && (t[i] == '+' || t[i] == '-') {
				i++
			}
			if run(isDigit) == 0 {
				fail("exponent requires digits")
				return token.FLOAT_LIT
			}
		}
		if !isFloat && t[0] == '0' {
			for j := 1; j < i; j++ {
				if t[j] > '7' {
					fail("malformed octal constant") // 0779: one run, one report
					return token.INT_LIT
				}
			}
		}
	}

	// The suffix is not checked here.
	//
	// A run that fails to be a constant is still a legal preprocessing
	// token (§2.3's PPNumber), and this scanner runs over token sequences
	// no phase takes a value from — an attribute's arguments are §8's
	// BalancedTokenSequence, and Apple writes a version number in one:
	//
	//	__attribute__((availability(macosx,introduced=10_2)))
	//
	// where `10_2` is a pp-number and not an integer constant, and saying
	// so would be four hundred diagnostics about code that means what it
	// says. The same reasoning already governs the two-dot case above.
	//
	// `int x = 1_024;` is still an error. It is reported where the value is
	// decoded — analyzer.DecodeIntConst — which is the phase that needs the
	// run to be a constant and the only one entitled to complain that it is
	// not.
	if isFloat {
		return token.FLOAT_LIT
	}
	return token.INT_LIT
}

// validIntSuffix: at most one unsigned part and one long part, in
// either order; ll must not mix case. ul, llu, ULL pass; lul, lL fail.
func validIntSuffix(t []byte) bool {
	var u, l bool
	for i := 0; i < len(t); {
		switch {
		case (t[i] == 'u' || t[i] == 'U') && !u:
			u = true
			i++
		case (t[i] == 'l' || t[i] == 'L') && !l:
			l = true
			if i+1 < len(t) && t[i+1] == t[i] {
				i++
			}
			i++
		default:
			return false
		}
	}
	return true
}

// scanChar scans a character constant; s.off is at the opening quote,
// start covers any prefix. ” is reported (at least one CChar
// required); the multi-character 'ab' scans clean — its value is a
// decoding concern. Unterminated is a formation error and reports in
// every mode; emptiness is a value error and defers under ScanPP.
func (s *scanner) scanChar(start int) {
	terminated, n := s.scanQuoted('\'')
	switch {
	case !terminated:
		s.errTok(start, s.off, "unterminated character constant")
	case n == 0:
		s.valueErr(start, s.off, "empty character constant")
	}
	s.emit(token.CHAR_LIT, start)
}

// scanString scans one string literal. Adjacent literals are not
// concatenated — that is §6.1's StringLiteralSequence, above this
// package, and it is where an @ on one piece reaches the others.
func (s *scanner) scanString(start int) {
	terminated, _ := s.scanQuoted('"')
	if !terminated {
		s.errTok(start, s.off, "unterminated string literal")
	}
	s.emit(token.STRING_LIT, start)
}

// scanQuoted consumes from the opening quote through the closing one.
// A raw line terminator is exactly what it looks like — phase 2
// already spliced backslash-newlines — so it ends the literal:
// reported once by the caller, token still emitted.
func (s *scanner) scanQuoted(quote byte) (terminated bool, n int) {
	s.off++ // opening quote
	for s.off < len(s.text) {
		switch c := s.text[s.off]; c {
		case quote:
			s.off++
			return true, n
		case '\n', '\r':
			return false, n
		case '\\':
			s.scanEscape()
			n++
		default:
			s.off++
			n++
		}
	}
	return false, n
}

func (s *scanner) scanEscape() {
	start := s.off
	s.off++ // backslash
	if s.off >= len(s.text) {
		return
	}
	c := s.text[s.off]
	switch {
	case c == '\n' || c == '\r':
		return // unterminated literal; the caller reports
	case strings.IndexByte(`'"?\abfnrtv`, c) >= 0:
		s.off++
	case c >= '0' && c <= '7':
		s.off++
		for n := 1; n < 3 && s.off < len(s.text) && s.text[s.off] >= '0' && s.text[s.off] <= '7'; n++ {
			s.off++
		}
	case c == 'x':
		s.off++
		if n := 0; true {
			for s.off < len(s.text) && isHexDigit(s.text[s.off]) {
				s.off++
				n++
			}
			if n == 0 {
				s.valueErr(start, s.off, `\x escape requires hexadecimal digits`)
			}
		}
	case c == 'u' || c == 'U':
		s.off = start
		s.scanUCN()
	default:
		s.off++
		s.valueErr(start, s.off, fmt.Sprintf("unknown escape sequence '\\%c'", c))
	}
}

// dots counts the '.' characters in a numeric run.
func dots(t []byte) int {
	n := 0
	for _, c := range t {
		if c == '.' {
			n++
		}
	}
	return n
}
