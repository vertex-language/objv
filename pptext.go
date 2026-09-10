package objv

import (
	"io"
	"sort"
	"strings"
	"sync"

	"github.com/vertex-language/objv/preprocessor"
	"github.com/vertex-language/objv/scanner"
	"github.com/vertex-language/objv/token"
)

// srcSpan maps one span of the reparsed text back to where it was written.
//
// Phase 4's output is printed and re-scanned so the parser can read it (see
// reparse), which puts every position phases 5-7 report in that printed text
// rather than in the file the user typed. A two-line program that imports
// Foundation reports its own second line as line 22,000 of itself. The
// mapping is recorded as the text is printed, where both ends are known.
type srcSpan struct {
	lo, hi int32 // offsets in the printed text
	site   preprocessor.Site
}

// srcMap is the spans of one printed stream, in ascending order — which is
// the order they were written, so the lookup is a binary search.
type srcMap []srcSpan

// at finds the span covering an offset, or the one nearest before it.
func (m srcMap) at(off int32) (srcSpan, bool) {
	i := sort.Search(len(m), func(i int) bool { return m[i].hi > off })
	if i == len(m) {
		return srcSpan{}, false
	}
	return m[i], true
}

// site maps a span of the printed text back to a site in the source.
//
// The start decides the file: a span that begins in one file and ends in
// another is a macro expansion straddling the two, and the end is dropped
// rather than producing a span no file contains. Where both ends are in the
// same file the whole extent is kept, so an expression underlines as an
// expression and not as its first token.
func (m srcMap) site(lo, hi int32) (preprocessor.Site, bool) {
	first, ok := m.at(lo)
	if !ok || !first.site.Valid() {
		return preprocessor.Site{}, false
	}
	out := first.site
	if hi > lo {
		if last, ok := m.at(hi - 1); ok && last.site.Origin == out.Origin &&
			last.site.End > out.Pos {
			out.End = last.site.End
		}
	}
	return out, true
}

// printOpts is what the two consumers of printTokens disagree about. They
// agree about everything else, which is why there is one printer.
type printOpts struct {
	// srcMap, when non-nil, collects one span per token as the stream is
	// printed. The reparse bridge asks for it; --emit mi does not, since its
	// output is the artifact rather than an intermediate to map back
	// through.
	srcMap *srcMap
}

// printTokens writes a phase-4 token stream back out as Objective-C source.
//
// Two rules, and the second is the one that matters. Line structure follows
// the tokens that came straight from a file: a macro's replacement list may
// have been written across several lines in its #define, and honouring those
// newlines would scatter the output for no gain.
//
// The other rule is paste avoidance. `#define PLUS +` then `+PLUS` must print
// `+ +`, never `++`, or the output does not re-enter as the same program.
//
// Pragma lines are kept, both here and on the reparse path. Phase 4 forwards
// the pragmas it does not act on, deliberately, because some of them mean
// something to phase 7 — #pragma pack changes the layout of every structure
// declared after it — and objv's scanner reads a #pragma as tokens rather
// than as trivia so that the parser can act on one. That is also what makes
// the promise `--emit mi` has to keep: the output re-enters as the same
// program, and a dropped pragma breaks it.
func printTokens(w io.Writer, toks []preprocessor.Token, opts printOpts) error {
	var b strings.Builder
	var prev preprocessor.Token
	have := false
	inPragma := false
	fromOperator := false // this pragma was written as _Pragma("…")

	for _, t := range toks {
		if t.Kind == token.EOF {
			continue
		}
		// A directive runs to the end of its line, so anything printed
		// after a #pragma is part of that pragma — and the token that
		// follows one is very often expanded, which is exactly the case the
		// newline rule below suppresses. Ending the line explicitly is what
		// stops a declaration after a pragma from becoming its arguments.
		//
		// A pragma written as _Pragma("…") is the case that makes this more
		// than a newline test. Its operands are generated, so they carry an
		// expansion of their own and cannot be told from a following
		// program token by that alone; what tells them apart is *whose*
		// expansion. NS_ASSUME_NONNULL_BEGIN is the operator spelling and
		// stands at the top of essentially every header in the SDK, so
		// getting this wrong turns `clang assume_nonnull begin` into a
		// declaration a few hundred times per compilation.
		endsPragma := inPragma && (t.Flags.Has(token.FlagNLBefore) ||
			(fromOperator && !fromPragmaOperator(t)))
		if endsPragma {
			inPragma, fromOperator = false, false
		}
		if opensPragmaLine(t) {
			inPragma = true
			fromOperator = false
		} else if inPragma && fromPragmaOperator(t) {
			fromOperator = true
		}

		switch {
		case !have:
		case endsPragma:
			b.WriteByte('\n')
		case t.Flags.Has(token.FlagNLBefore) && (t.Exp == nil || t.Kind == token.HASH):
			b.WriteByte('\n')
		case t.Spaced() || needSpace(prev, t):
			b.WriteByte(' ')
		}
		if opts.srcMap != nil {
			lo := int32(b.Len())
			*opts.srcMap = append(*opts.srcMap, srcSpan{
				lo: lo, hi: lo + int32(len(t.Text())), site: t.Site(),
			})
		}
		b.WriteString(t.Text())
		prev, have = t, true
	}
	if have {
		b.WriteByte('\n')
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// opensPragmaLine reports whether t is the generated '#' phase 4 mints for a
// pragma it passed through. Generated, so it belongs to no file, and at the
// start of a line, because that is where a directive begins.
func opensPragmaLine(t preprocessor.Token) bool {
	return t.Kind == token.HASH && t.Exp == nil &&
		t.Origin != nil && t.Origin.File == nil &&
		t.Flags.Has(token.FlagNLBefore)
}

// fromPragmaOperator reports whether a token was produced by §6.10.9's
// _Pragma — the spelling a macro has to use, since a macro cannot expand to a
// directive.
func fromPragmaOperator(t preprocessor.Token) bool {
	for e := t.Exp; e != nil; e = e.Outer {
		if e.Macro == "_Pragma" {
			return true
		}
	}
	return false
}

var (
	spaceMu    sync.Mutex
	spaceCache = map[string]bool{}
)

// needSpace reports whether two adjacent spellings would lex as something
// other than the two tokens themselves.
//
// This asks the scanner rather than consulting a character-class table,
// because the table is where every preprocessor's paste-avoidance bugs live:
// `.` `.` `.` is the obvious one, `>` `>=`, `+` `++`, hex-float `p` `+`, and
// in Objective-C `@` `"` — which is one token and not two. Same trick ##
// uses to validate a paste, and the result is cached because the same pair
// recurs constantly.
//
// The cache is shared and guarded: a compiler may be driven from a worker
// pool, and this is the one piece of state a translation unit does not own.
func needSpace(a, b preprocessor.Token) bool {
	joined := a.Text() + b.Text()
	spaceMu.Lock()
	v, ok := spaceCache[joined]
	spaceMu.Unlock()
	if ok {
		return v
	}
	f := token.NewFile("<paste-check>", []byte(joined+"\n"))
	toks, diags := scanner.Scan(f, 0)

	need := true
	if len(diags) == 0 && len(toks) == 3 && toks[2].Kind == token.EOF &&
		toks[0].Kind == a.Kind && toks[1].Kind == b.Kind &&
		string(f.Slice(toks[0].Pos, toks[0].End)) == a.Text() {
		need = false
	}
	spaceMu.Lock()
	spaceCache[joined] = need
	spaceMu.Unlock()
	return need
}
