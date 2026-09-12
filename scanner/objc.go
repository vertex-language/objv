package scanner

import (
	"fmt"

	"github.com/vertex-language/objv/token"
)

// scanAt scans the '@' punctuator, directives (§2.5), and string literals (§2.4).
// Whitespace after '@' is tolerated; comments between '@' and identifier are not merged.
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
		// @"…" is an OBJC_STRING_LIT token (§2.4).
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
		// Not a directive; emit standalone AT and let next iteration scan identifier.
		s.emit(token.AT, at)
		s.atIdentErr(at, end, name)

	default:
		// Boxed expressions and literals (§6.8): emit standalone AT.
		s.emit(token.AT, at)
	}
}

// atIdentErr reports an '@' prefix on an identifier that is not a directive (§2.5).
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
