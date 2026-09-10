package preprocessor

import (
	"io/fs"
	"path"
	"strings"

	"github.com/vertex-language/objv/token"
)

// Deps accumulates what --deps writes: the target, and every file the include
// graph reached, in first-seen order.
type Deps struct {
	Target string
	Files  []string
	seen   map[string]bool
}

func (d *Deps) add(name string) {
	if d == nil {
		return
	}
	if d.seen == nil {
		d.seen = map[string]bool{}
	}
	if !d.seen[name] {
		d.seen[name] = true
		d.Files = append(d.Files, name)
	}
}

// Write renders the Makefile fragment build systems consume: one rule, then a
// phony target per header so a deleted header does not break the rebuild. This
// is -MMD -MF -MP collapsed to the one shape that is actually read.
func (d *Deps) Write(w *strings.Builder) {
	w.WriteString(d.Target)
	w.WriteString(":")
	for _, f := range d.Files {
		w.WriteString(" \\\n  ")
		w.WriteString(escapeMake(f))
	}
	w.WriteString("\n")
	for _, f := range d.Files[1:] {
		w.WriteString("\n")
		w.WriteString(escapeMake(f))
		w.WriteString(":\n")
	}
}

func escapeMake(s string) string {
	return strings.NewReplacer(" ", `\ `, "#", `\#`, "$", "$$").Replace(s)
}

// cached is one entry of the open-once cache. Content is read at most once per
// translation unit; guard is the controlling macro discovered when the file
// was fully read, and is what lets a second #include skip the file entirely.
//
// diags holds the phases 1–3 diagnostics scanning produced, deferred: they are
// reported on the first read, through the real Origin, so they carry the
// inclusion chain and the System treatment — open() has neither.
type cached struct {
	file  *token.File
	toks  []Token
	diags []token.Diagnostic
	guard string
	done  bool

	// once is set when the file need not be read again whatever the macro
	// table says: it was reached by #import, or it said #pragma once while
	// being read. Unlike guard there is no macro standing behind the
	// request that the program could undefine.
	once bool
}

// includeMode distinguishes the three directives that read a file.
type includeMode uint8

const (
	incInclude includeMode = iota
	incIncludeNext
	incImport
)

func (m includeMode) word() string {
	switch m {
	case incIncludeNext:
		return "#include_next"
	case incImport:
		return "#import"
	}
	return "#include"
}

// doInclude implements §6.10.2 for #include, #include_next, and #import.
func (p *Preprocessor) doInclude(r *reader, mode includeMode, line []Token, at Site) {
	what := mode.word()
	name, angled, ok := p.headerName(what, line, at)
	if !ok {
		return
	}
	if len(p.stack) >= p.cfg.MaxIncludeDepth {
		p.errorf(at, "%s nested too deeply (limit %d)", what, p.cfg.MaxIncludeDepth)
		if mode == incInclude {
			p.note(at, "a header that includes itself needs an include guard, or #import")
		}
		return
	}
	p.include(r, name, angled, at, mode)
}

// headerName recovers the header-name pp-token, which phase 3 cannot produce:
// <Foundation/NSString.h> scans as LSS IDENT QUO IDENT DOT IDENT GTR
// everywhere except here, and only this call site knows the context.
//
// The quoted form is one STRING_LIT and needs no reconstruction. The angled
// form is rebuilt from the raw bytes between '<' and '>', not from the token
// spellings, because the characters between them are not tokens: the slash
// separating a framework from its header was never one.
func (p *Preprocessor) headerName(what string, line []Token, at Site) (name string, angled, ok bool) {
	if len(line) == 0 {
		p.errorf(at, "%s expects \"FILENAME\" or <FILENAME>", what)
		return "", false, false
	}
	switch {
	case line[0].Kind == token.STRING_LIT && !strings.HasPrefix(line[0].Text(), "u") &&
		!strings.HasPrefix(line[0].Text(), "L") && !strings.HasPrefix(line[0].Text(), "U"):
		p.expectEnd(line[1:], what)
		return strings.Trim(line[0].Text(), `"`), false, true

	case line[0].Kind == token.OBJC_STRING_LIT:
		// @"Cache.h" is a string object, not a header name. It is worth its
		// own diagnostic because the @ is one keystroke away from every
		// other line in the file.
		p.errorf(line[0].Site(), "%s expects \"FILENAME\" or <FILENAME>", what)
		p.note(line[0].Site(), "@\"…\" is an Objective-C string object; drop the @")
		return "", false, false

	case line[0].Kind == token.LSS:
		for i := 1; i < len(line); i++ {
			if line[i].Kind != token.GTR {
				continue
			}
			org := line[0].Origin
			if org == nil || org.File == nil {
				break
			}
			p.expectEnd(line[i+1:], what)
			return string(org.File.Slice(line[0].End, line[i].Pos)), true, true
		}
		p.errorf(at, "missing '>' in %s", what)
		return "", false, false
	}

	// Neither form: the line is a macro that expands to one. §6.10.2p4.
	expanded := p.expandClosed(line)
	if len(expanded) > 0 && !sameTokens(expanded, line) {
		return p.headerName(what, expanded, at)
	}
	p.errorf(at, "%s expects \"FILENAME\" or <FILENAME>", what)
	return "", false, false
}

func sameTokens(a, b []Token) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Kind != b[i].Kind || a[i].Text() != b[i].Text() {
			return false
		}
	}
	return true
}

// place is one candidate location: a mount and a path within it.
type place struct {
	mount *Mount
	rel   string
}

// include resolves and reads a header.
//
// The search is two lists. A quoted include looks in the including file's own
// directory first; then both forms walk Search, and then Frameworks. That is
// §6.10.2 with one Objective-C addition, and no -iquote/-isystem/-idirafter
// tower layered on top.
func (p *Preprocessor) include(r *reader, name string, angled bool, at Site, mode includeMode) {
	what := mode.word()
	if path.IsAbs(name) {
		p.errorf(at, "absolute path in %s: %q", what, name)
		p.note(at, "use -I, -F and a relative path so the build is reproducible")
		return
	}

	// #include_next resumes the list after the directory this file was found
	// in, so a header can reach the one it shadows. A file found outside the
	// search list — the primary source — has no position in it, and the
	// search runs from the top.
	start := p.searchStart(r, mode == incIncludeNext)

	for _, pl := range p.searchList(r, name, angled, mode, start) {
		rel := path.Clean(pl.rel)
		if strings.HasPrefix(rel, "..") {
			continue
		}
		display := path.Join(pl.mount.Name, rel)
		c, err := p.open(pl.mount, rel, display)
		if err != nil {
			continue
		}
		p.deps.add(display)

		// A file need not be opened again when it said so, or when it is
		// shaped like a header that says so:
		//
		//   - #import asked for it outright, and so did #pragma once;
		//   - an #ifndef guard says the same thing conditionally, and holds
		//     only while its macro is defined.
		//
		// The first is why Objective-C code has no guards to write: the
		// directive states the conclusion the guard would let a compiler
		// infer.
		if c.done && (c.once || (c.guard != "" && p.macros.Defined(c.guard))) {
			if mode == incImport {
				c.once = true
			}
			return
		}
		p.readFile(c, pl.mount, rel, display, at, r.org, mode)
		return
	}

	if mode == incIncludeNext {
		// Nothing further down the list has it, which is what a wrapper
		// header at the bottom of the list will find. Saying nothing is
		// wrong; naming it as not found is right.
		p.errorf(at, "%q file not found after this directory", name)
		return
	}
	p.errorf(at, "%q file not found", name)
	p.notFoundNote(at, name)
}

// notFoundNote says which list came up empty, because the fix differs: a
// framework include that found no framework needs a -F, and everything else
// needs a -I.
func (p *Preprocessor) notFoundNote(at Site, name string) {
	switch fw, _, isFramework := splitFramework(name); {
	case len(p.cfg.Search) == 0 && len(p.cfg.Frameworks) == 0:
		p.note(at, "no include or framework directories are configured; use -I and -F")
	case isFramework && len(p.cfg.Frameworks) == 0:
		p.note(at, "no framework directories are configured; %s.framework would be found through -F", fw)
	default:
		p.note(at, "searched %d include and %d framework directories; run `objv env` to see the resolved lists",
			len(p.cfg.Search), len(p.cfg.Frameworks))
	}
}

// searchList is where a header of this name would be looked for, in order.
//
// Apart from include() because __has_include asks the same question and must
// get the same answer. A rule that decided where to look twice would answer
// the question differently from the directive it guards, which is the one
// thing __has_include may not do.
func (p *Preprocessor) searchList(r *reader, name string, angled bool, mode includeMode, start int) []place {
	var out []place
	next := mode == incIncludeNext
	if !angled && !next && r.org.Mount != nil {
		out = append(out, place{r.org.Mount, path.Join(path.Dir(r.org.Path), name)})
	}
	for i := start; i < len(p.cfg.Search); i++ {
		out = append(out, place{&p.cfg.Search[i], name})
	}

	// A framework include names a framework and a header inside it. The
	// including file's own framework is not searched separately: a header in
	// Foundation.framework/Headers that writes #import <Foundation/NSArray.h>
	// finds it through the same framework directory Foundation was found in,
	// which is already in this list.
	fw, rest, ok := splitFramework(name)
	if !ok {
		return out
	}
	for i := range p.cfg.Frameworks {
		m := &p.cfg.Frameworks[i]
		out = append(out,
			place{m, path.Join(fw+".framework", "Headers", rest)},
			place{m, path.Join(fw+".framework", "PrivateHeaders", rest)})
	}
	return out
}

// splitFramework splits <Foundation/NSString.h> into its framework name and
// the path inside it. A name with no slash names no framework, and neither
// does one whose first component is empty.
func splitFramework(name string) (framework, rest string, ok bool) {
	i := strings.IndexByte(name, '/')
	if i <= 0 || i+1 >= len(name) {
		return "", "", false
	}
	return name[:i], name[i+1:], true
}

// searchStart is the index #include_next resumes the list at: one past the
// entry the including file was found in. An ordinary include starts at zero,
// and so does one from a file that has no position in the list.
func (p *Preprocessor) searchStart(r *reader, next bool) int {
	if !next {
		return 0
	}
	for i := range p.cfg.Search {
		if r.org.Mount == &p.cfg.Search[i] {
			return i + 1
		}
	}
	return 0
}

// headerExists reports whether an include of this name would find a file.
//
// It stats rather than opens: the header is not being read, so nothing is
// scanned, nothing is cached, and nothing joins the dependency list. A header
// a program asked about and did not import is not a file the build depends on.
func (p *Preprocessor) headerExists(r *reader, name string, angled, next bool) bool {
	if path.IsAbs(name) {
		return false
	}
	mode := incInclude
	if next {
		mode = incIncludeNext
	}
	for _, pl := range p.searchList(r, name, angled, mode, p.searchStart(r, next)) {
		rel := path.Clean(pl.rel)
		if strings.HasPrefix(rel, "..") {
			continue
		}
		// Already read this translation unit: it exists, whatever the
		// filesystem says now.
		if _, ok := p.files[path.Join(pl.mount.Name, rel)]; ok {
			return true
		}
		if _, err := fs.Stat(pl.mount.FS, rel); err == nil {
			return true
		}
	}
	return false
}

// open reads a file at most once per translation unit. Scanning diagnostics
// are stashed, not reported: they wait for readFile, where an Origin with the
// inclusion chain exists.
func (p *Preprocessor) open(m *Mount, rel, display string) (*cached, error) {
	if c, ok := p.files[display]; ok {
		return c, nil
	}
	src, err := fs.ReadFile(m.FS, rel)
	if err != nil {
		return nil, err
	}
	f := token.NewFile(display, src)
	toks, diags := scanPP(f)
	c := &cached{file: f, diags: diags}
	c.toks = p.wrap(toks, nil) // origin attached per-read, below
	p.files[display] = c
	return c, nil
}

func (p *Preprocessor) readFile(c *cached, m *Mount, rel, display string, at Site, parent *Origin, mode includeMode) {
	org := &Origin{
		File:       c.file,
		Mount:      m,
		Path:       rel,
		Parent:     parent,
		IncludePos: at.Pos,
		System:     m.System,
		Guard:      c.guard,
	}
	toks := make([]Token, len(c.toks))
	copy(toks, c.toks)
	for i := range toks {
		toks[i].Origin = org
	}
	r := &reader{org: org, toks: toks, miValid: true}

	// Phases 1–3 diagnostics, deferred from open(): reported here, on the
	// first read only, through the real Origin — so they carry the include
	// chain and the System treatment. A scanner mistake in a header is the
	// header's, once, like any other diagnostic.
	if !c.done {
		for _, d := range c.diags {
			p.fromScan(org, d)
		}
	}

	p.stack = append(p.stack, r)
	p.run(r)
	p.stack = p.stack[:len(p.stack)-1]

	// Record what we learned, so the next include of this file can skip it.
	if !c.done {
		c.done = true
		c.guard = r.guardFound
		c.once = r.once
	}
	if mode == incImport {
		c.once = true
	}
}
