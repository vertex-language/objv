package lower

// Lowering for inline assembly (§7.4).
//
// Supports basic/operandless inline assembly (such as compiler memory barriers).
// Extended inline assembly with operand register constraints is reported as unsupported.

import (
	"strings"

	"github.com/vertex-language/objv/analyzer"
	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/token"
)

func (u *unit) asmStmt(s *ast.AsmStmt) {
	if !u.at() {
		return
	}
	template, clobbers, ok := u.asmParts(s)
	if !ok {
		return
	}
	stmt := u.fn.cur.Asm(template)
	// Volatile whether or not it was written. A statement with no outputs
	// is volatile by GCC's own rule, and every statement this lowers has
	// none.
	stmt.Volatile()
	if len(clobbers) > 0 {
		stmt.Clobber(clobbers...)
	}
	stmt.Emit()
}

// asmParts reads the template and the clobber list out of the token
// sequence, and refuses anything with an operand in it.
func (u *unit) asmParts(s *ast.AsmStmt) (template string, clobbers []string, ok bool) {
	if s.Body == nil {
		return "", nil, false
	}
	toks := s.Body.List

	// The template: one or more adjacent string literals, concatenated as
	// §5.1.1.2 concatenates any others.
	i := 0
	for ; i < len(toks) && toks[i].Kind == token.STRING_LIT; i++ {
		text, err := u.asmString(toks[i])
		if err {
			return "", nil, false
		}
		template += text
	}
	if i == 0 {
		u.unsupported(s, "an inline assembly statement with no template")
		return "", nil, false
	}

	// Then up to four sections. Only the third holds anything this can
	// lower, and only a string in it.
	section := 0
	for ; i < len(toks); i++ {
		switch toks[i].Kind {
		case token.COLON:
			section++
			continue
		case token.STRING_LIT:
			if section != 3 {
				u.unsupported(s, "an inline assembly operand")
				return "", nil, false
			}
			text, err := u.asmString(toks[i])
			if err {
				return "", nil, false
			}
			clobbers = append(clobbers, text)
			continue
		case token.COMMA:
			continue
		}
		u.unsupported(s, "an inline assembly operand")
		return "", nil, false
	}
	return template, clobbers, true
}

// asmString decodes one string literal of the body.
func (u *unit) asmString(t token.Token) (string, bool) {
	lit := &ast.StringLit{Segs: []ast.Span{{Lo: t.Pos, Hi: t.End}}}
	bad := false
	val := analyzer.DecodeString(u.src, lit, u.model, func(string) { bad = true })
	if bad {
		return "", true
	}
	var b strings.Builder
	for _, c := range val.Data[:max(len(val.Data)-1, 0)] {
		b.WriteByte(byte(c))
	}
	return b.String(), false
}
