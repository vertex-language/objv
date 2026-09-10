package scanner

import (
	"fmt"

	"github.com/vertex-language/objv/token"
)

// scanAt scans the @ punctuator and the two longer tokens it heads: a
// Directive (§2.5) and an ObjectStringLiteral (§2.4).
//
// §2 notes that an implementation will generally tolerate whitespace
// or a comment between the @ and the identifier. objv tolerates the
// whitespace. It does not tolerate a comment, because a comment
// between them has nowhere to go: swallowed into the token's span it
// is a COMMENT token that ScanComments promised and did not deliver,
// and emitted on its own it is a token sitting inside another token's
// span. `@ /* why */ interface` is therefore an @ and an identifier,
// and the parser reports the @ that heads nothing.
func (s *scanner) scanAt() {
	at := s.off
	s.off++ // @

	gap := s.off
	for gap < len(s.text) && isSpace(s.text[gap]) {
		gap++
	}
	var fl token.Flags
	if gap > s.off {
		fl = token.FlagSpacedAt
	}
	if gap >= len(s.text) {
		s.emit(token.AT, at)
		return
	}

	switch c := s.text[gap]; {
	case c == '"':
		// @"…" is one token and one pointer to a string object, where
		// "…" alone is an array of characters. §6.1 lets a sequence
		// that starts with this one continue in either spelling.
		s.off = gap
		if terminated, _ := s.scanQuoted('"'); !terminated {
			s.errTok(at, s.off, "unterminated string literal")
		}
		s.emitFlags(token.OBJC_STRING_LIT, at, fl)

	case s.identStart(c):
		end := gap + 1
		for end < len(s.text) && s.identPart(s.text[end]) {
			end++
		}
		name := string(s.text[gap:end])
		if k := token.LookupDirective(name); k != token.ILLEGAL {
			s.off = end
			s.emitFlags(k, at, fl)
			return
		}
		// Not a directive, and the set is closed. The @ stands alone
		// and the identifier is scanned by the next turn of the loop:
		// @__objc_yes and @__objc_no are §6.8's boxed BooleanConstant
		// and are well-formed, so only the rest is reported.
		s.emit(token.AT, at)
		s.atIdentErr(at, end, name)

	default:
		// @( @[ @{ @42 @-1 @'c' — §6.8's boxing and collection
		// literals, each of which is an @ and then an ordinary token.
		s.emit(token.AT, at)
	}
}

// atIdentErr reports an @ on an identifier that §2.5's closed list does
// not name. It defers under ScanPP like any other value-level report:
// an excluded #if group may hold an @ on anything.
func (s *scanner) atIdentErr(at, end int, name string) {
	switch name {
	case "__objc_yes", "__objc_no":
		return // @YES and @NO, expanded
	case "true", "false":
		s.valueErr(at, end, fmt.Sprintf(
			"@%s is Objective-C++ only; write @%s in Objective-C",
			name, map[string]string{"true": "YES", "false": "NO"}[name]))
	default:
		// The closed list is also why an enumeration constant cannot be
		// boxed by name (§6.8), which is what this almost always is.
		s.valueErr(at, end, fmt.Sprintf(
			"@%s is not a directive; to box a constant write @(%s)", name, name))
	}
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\v' || c == '\f' || c == '\n' || c == '\r'
}
