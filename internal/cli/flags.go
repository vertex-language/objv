package cli

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/vertex-language/objv"
	"github.com/vertex-language/objv/preprocessor"
	"github.com/vertex-language/objv/sysroot"
)

// Flag set definitions and Compiler construction from CLI arguments.

type stringList []string

func (s *stringList) String() string     { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error { *s = append(*s, v); return nil }

// defineFlag appends to a shared slice so -D and -U keep command-line order.
type defineFlag struct {
	list *[]preprocessor.Predefine
	kind preprocessor.PredefineKind
}

func (d defineFlag) String() string { return "" }
func (d defineFlag) Set(v string) error {
	*d.list = append(*d.list, preprocessor.Predefine{Kind: d.kind, Text: v})
	return nil
}

// ppFlags is the front-end flag set, shared by every verb that reads source.
type ppFlags struct {
	includes     stringList
	frameworks   stringList
	defines      []preprocessor.Predefine
	preInc       stringList
	target       string
	sdk          string
	minOS        string
	arc          bool
	freestanding bool
	force        bool // -pp: preprocess even when the extension says otherwise
	raw          bool // -no-pp: never preprocess
	keepComments bool // set by cmdAST, not a flag: -comments implies it

	c *objv.Compiler // built at most once, so the SDK walk happens once
}

func (p *ppFlags) register(fs *flag.FlagSet) {
	fs.Var(&p.includes, "I", "add an include search directory (repeatable, in order)")
	fs.Var(&p.frameworks, "F", "add a framework search directory (repeatable, in order)")
	fs.Var(defineFlag{&p.defines, preprocessor.PredefineDefine}, "D", "define a macro (repeatable)")
	fs.Var(defineFlag{&p.defines, preprocessor.PredefineUndef}, "U", "undefine a macro (repeatable)")
	fs.Var(&p.preInc, "include", "process a file before the main input (repeatable)")
	fs.StringVar(&p.target, "target", objv.HostName(), "target to compile for")
	fs.StringVar(&p.sdk, "isysroot", "", "the SDK to compile and link against")
	fs.StringVar(&p.minOS, "mmacosx-version-min", "", "the oldest macOS this build runs on")
	fs.BoolVar(&p.arc, "fobjc-arc", false, "compile with automatic reference counting")
	fs.BoolVar(&p.freestanding, "freestanding", false, "builtin headers only; no SDK, no frameworks")
	fs.BoolVar(&p.force, "pp", false, "preprocess regardless of extension (for stdin)")
	fs.BoolVar(&p.raw, "no-pp", false, "input is already preprocessed")
}

// compiler validates flags and constructs a new *objv.Compiler.
func (p *ppFlags) compiler() (*objv.Compiler, error) {
	if p.c != nil {
		return p.c, nil
	}
	if p.target == "" {
		return nil, fmt.Errorf(
			"this host is not a target objv models; name one with -target (known: %s)",
			strings.Join(objv.Targets(), ", "))
	}
	if _, ok := objv.LookupTarget(p.target); !ok {
		return nil, fmt.Errorf("unknown target %q (known: %s)",
			p.target, strings.Join(objv.Targets(), ", "))
	}
	var deployment sysroot.Version
	if p.minOS != "" {
		v, ok := sysroot.ParseVersion(p.minOS)
		if !ok {
			return nil, fmt.Errorf("-mmacosx-version-min: not a version: %q", p.minOS)
		}
		deployment = v
	}
	epoch, err := epoch()
	if err != nil {
		return nil, err
	}
	p.c = &objv.Compiler{
		Target:        p.target,
		IncludeDirs:   p.includes,
		FrameworkDirs: p.frameworks,
		Defines:       p.defines,
		PreIncludes:   p.preInc,
		SDKPath:       p.sdk,
		Deployment:    deployment,
		ARC:           p.arc,
		Freestanding:  p.freestanding,
		KeepComments:  p.keepComments,
		PP:            p.pp(),
		SourceDate:    epoch,
	}
	return p.c, nil
}

// pp is what -pp and -no-pp say about phase 4. Neither is the extension's
// answer, which is what the library's zero value already means.
func (p *ppFlags) pp() objv.Tristate {
	switch {
	case p.raw:
		return objv.PPNever
	case p.force:
		return objv.PPAlways
	}
	return objv.PPAuto
}

// epoch reads SOURCE_DATE_EPOCH or defaults to Unix epoch for deterministic builds.
func epoch() (*time.Time, error) {
	if s := os.Getenv("SOURCE_DATE_EPOCH"); s != "" {
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("SOURCE_DATE_EPOCH: not a decimal timestamp: %q", s)
		}
		t := time.Unix(n, 0).UTC()
		return &t, nil
	}
	t := time.Unix(0, 0).UTC()
	return &t, nil
}

// buildFlags is `objv build`, and — minus --emit and -o — `objv run`.
type buildFlags struct {
	pp   ppFlags
	emit string
	out  string

	libDirs stringList
	libs    stringList
	fws     stringList
	entry   string
	static  bool
}

func (b *buildFlags) register(fs *flag.FlagSet) {
	b.pp.register(fs)
	fs.Var(&b.libDirs, "L", "add a library search directory (repeatable, in order)")
	fs.Var(&b.libs, "l", "link against a library (repeatable, in order)")
	fs.Var(&b.fws, "framework", "link against a framework (repeatable, in order)")
	fs.StringVar(&b.entry, "entry", "", "the program's entry symbol (default: the platform's)")
	fs.BoolVar(&b.static, "static", false, "link a static image")
}

// params is the build the flags describe, over inputs in command-line order.
//
// -F reaches the link as well as the compilation: a framework's headers and
// its stub live in one directory, and a caller who named one for the first
// meant it for the second.
func (b *buildFlags) params(ins []objv.Input, out string) objv.BuildParams {
	return objv.BuildParams{
		Output:      out,
		Inputs:      ins,
		Libraries:   b.libs,
		LibraryDirs: b.libDirs,
		Frameworks:  b.fws,
		Entry:       b.entry,
		Static:      b.static,
	}
}
