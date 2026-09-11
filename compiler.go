package objv

import (
	"fmt"
	"io/fs"
	"os"
	"sync"
	"time"

	"github.com/vertex-language/objv/preprocessor"
	"github.com/vertex-language/objv/sysroot"
)

// Tristate is a three-valued switch: follow the input, or override it.
type Tristate uint8

const (
	PPAuto   Tristate = iota // .m is preprocessed, .mi is not
	PPAlways                 // preprocess regardless of extension
	PPNever                  // the input is already preprocessed
)

// A Compiler is a target, a search list, and the machine to ask about both.
//
// The zero value compiles for this host with no extra directories, which is
// what `objv build` does with no flags:
//
//	var c objv.Compiler
//	err := c.Build(objv.BuildParams{
//		Output: "hello",
//		Inputs: []objv.Input{objv.File("hello.m")},
//	})
//
// The configuration a target implies — its predefined macros, the include and
// framework search lists under it, the SDK, the deployment target — is
// resolved once, on first use, and reused for every input after. That is
// deliberate on both counts: resolving it eagerly would break a caller who
// only wants to parse .mi source on a machine objv cannot model, and
// resolving it per file would run xcrun once per translation unit, which is
// fine for a command compiling one file and wrong for a build system
// compiling four hundred.
//
// A Compiler must not be mutated or copied after its first call. Concurrent
// calls on one Compiler are otherwise fine: everything a phase touches after
// that point is either read-only or per-unit.
type Compiler struct {
	// Target names the machine to compile for: "aarch64-macos", and so on
	// through Targets. "" is this host, and is an error on a host objv does
	// not model — where a caller says which target it meant.
	Target string

	// IncludeDirs are searched before everything sysroot resolves, in order.
	IncludeDirs []string

	// FrameworkDirs are searched before the SDK's, in order. It is -F, and
	// it is the flag a program with a framework of its own needs.
	FrameworkDirs []string

	// Defines are -D and -U in one list, because their order is meaning:
	// `-D FOO -U FOO -D FOO=2` has to say what it says. The target's own
	// predefines precede all of them, so a caller can undefine one.
	Defines []preprocessor.Predefine

	// PreIncludes are processed before the main input, in order.
	PreIncludes []string

	// SDKPath overrides SDK discovery, as clang's -isysroot does.
	SDKPath string

	// Deployment overrides the deployment target, as -mmacosx-version-min
	// does. The zero Version lets sysroot decide.
	Deployment sysroot.Version

	// ARC compiles with automatic reference counting, as -fobjc-arc does.
	// It is not a default: a program written before ARC compiles only
	// without it, and one written for ARC does not compile without it, so
	// there is no setting that is right for both and the caller says.
	ARC bool

	// Freestanding compiles against objv's builtin headers alone: no SDK, no
	// frameworks, and nothing linked that the caller did not name.
	Freestanding bool

	// KeepComments keeps COMMENT tokens through phase 4, for a caller that
	// asked the parser to retain them.
	KeepComments bool

	// PP decides whether phase 4 runs. The zero value follows the input's
	// extension.
	PP Tristate

	// SourceDate fixes __DATE__ and __TIME__. Nil is the Unix epoch, so a
	// build is reproducible with nothing said.
	//
	// Deliberately not read from SOURCE_DATE_EPOCH here: a library that
	// reads the environment behaves differently in a test than in a
	// terminal. The command line reads the variable and passes it in.
	SourceDate *time.Time

	// Host is every impurity behind header and SDK discovery — the
	// environment, directory existence, xcrun. Nil is the real machine. A
	// caller that supplies one gets a compiler that touches nothing outside
	// it, which is what makes a hermetic build hermetic and a test a test.
	Host sysroot.Host

	// OnDiagnostic receives every diagnostic as it is produced, warnings
	// included, in the order a compiler would print them. Nil is fine: each
	// phase returns its diagnostics anyway, and Build collects errors into a
	// *DiagnosticError.
	OnDiagnostic func(Diagnostic)

	// Producer is the toolchain string stamped into object files and the
	// .vir banner. "" is "objv " + Version.
	Producer string

	once  sync.Once
	cfg   preprocessor.Config
	res   sysroot.Result
	cfgTt Target
	cfgEr error
}

// An Env is the resolved configuration: what an #import will search, in the
// order it walks, and what the target defines before the first line is read.
//
// It is what `objv env` prints, and the point of it is the invariant the
// READMEs promise — header search is data, inspectable before the build runs.
// In Objective-C that matters more than in C: a program that fails to find
// Foundation fails at its first line, and the question is always which of
// four SDKs it looked in.
type Env struct {
	Target     Target
	Hosted     bool
	ARC        bool
	SDK        sysroot.SDK
	Deployment sysroot.Version

	Search     []preprocessor.Mount
	Frameworks []preprocessor.Mount
	Predefines []preprocessor.Predefine

	// LibraryDirs is where a -l name is looked for, in order, and Libraries
	// is what a hosted link gets when the caller named none. The link's half
	// of Search.
	LibraryDirs []string
	Libraries   []string

	// Notes is sysroot's advice where something expected was missing: no
	// SDK, no GNUstep. They are never errors — a host with no system headers
	// still resolves, to the builtins.
	Notes []string
}

// Env resolves the configuration and reports it.
func (c *Compiler) Env() (Env, error) {
	cfg, r, err := c.config()
	if err != nil {
		return Env{}, err
	}
	t, _ := c.target()
	return Env{
		Target:      t,
		Hosted:      !c.Freestanding,
		ARC:         c.ARC,
		SDK:         r.SDK,
		Deployment:  r.Deployment,
		Search:      cfg.Search,
		Frameworks:  cfg.Frameworks,
		Predefines:  cfg.Predefines,
		LibraryDirs: r.LibraryDirs,
		Libraries:   r.Libraries,
		Notes:       r.Notes,
	}, nil
}

// target resolves the target name.
//
// It is separate from config because a caller may need the type model without
// needing a search list — and because indexing the table without checking
// gives a zero Model, which sizes every type at nothing and reports nonsense.
func (c *Compiler) target() (Target, error) {
	name := c.Target
	if name == "" {
		name = HostName()
	}
	if name == "" {
		return Target{}, fmt.Errorf(
			"this host is not a target objv models; name one (known: %s)", targetList())
	}
	t, ok := LookupTarget(name)
	if !ok {
		return Target{}, fmt.Errorf("unknown target %q (known: %s)", name, targetList())
	}
	return t, nil
}

// config composes phase 4's configuration, once.
//
// The order of Search is the one preprocessor's config.go documents:
// IncludeDirs first, then what sysroot resolved. The order of Predefines is
// the target's model, then the platform's, then the caller's -D and -U — so
// a caller can undefine any of them and nothing needs a mechanism for it.
func (c *Compiler) config() (preprocessor.Config, sysroot.Result, error) {
	c.once.Do(func() {
		t, err := c.target()
		if err != nil {
			c.cfgEr = err
			return
		}
		c.cfgTt = t

		opt := sysroot.Options{
			Target:     t.Name(),
			Hosted:     !c.Freestanding,
			SDKPath:    c.SDKPath,
			Deployment: c.Deployment,
		}
		r := sysroot.ResolveWith(c.Host, opt)
		c.res = r

		cfg := t.ppConfig(r)
		// __STDC_HOSTED__, which the builtin headers read: a hosted
		// <limits.h> defers to the platform's and a freestanding one is
		// the whole of what there is.
		cfg.Hosted = !c.Freestanding

		// -I and -F precede everything sysroot found. A caller's directory
		// is not System: a warning in one is theirs to see every time.
		var search []preprocessor.Mount
		for _, dir := range c.IncludeDirs {
			search = append(search, preprocessor.Mount{Name: dir, FS: dirFS(dir)})
		}
		cfg.Search = append(search, cfg.Search...)

		var frameworks []preprocessor.Mount
		for _, dir := range c.FrameworkDirs {
			frameworks = append(frameworks, preprocessor.Mount{Name: dir, FS: dirFS(dir)})
		}
		cfg.Frameworks = append(frameworks, cfg.Frameworks...)

		cfg.Predefines = append(cfg.Predefines, t.Predefines()...)
		for _, d := range sysroot.Predefines(opt, r) {
			cfg.Predefines = append(cfg.Predefines, preprocessor.Predefine{Text: d})
		}
		// ARC is a fact about this compilation that a header may ask about
		// through __has_feature — which is answered by hasFeature, above,
		// and cannot see this — so it is also stated as a macro, the way
		// clang states it.
		if c.ARC {
			cfg.Predefines = append(cfg.Predefines,
				preprocessor.Predefine{Text: "__OBJC_ARC__=1"})
		}
		cfg.Predefines = append(cfg.Predefines, c.Defines...)

		cfg.PreIncludes = c.PreIncludes
		cfg.KeepComments = c.KeepComments
		cfg.Epoch = c.SourceDate
		c.cfg = cfg
	})
	return c.cfg, c.res, c.cfgEr
}

// report hands each diagnostic to OnDiagnostic and returns the list, so a
// caller may take either route without the two disagreeing about order.
func (c *Compiler) report(diags []Diagnostic) []Diagnostic {
	if c.OnDiagnostic != nil {
		for _, d := range diags {
			c.OnDiagnostic(d)
		}
	}
	return diags
}

// producer is the toolchain string stamped where a container has a place
// for it.
func (c *Compiler) producer() string {
	if c.Producer != "" {
		return c.Producer
	}
	return "objv " + Version
}

// dirFS is os.DirFS with the empty path handled: a -I with no directory is a
// mistake worth ignoring rather than a panic.
func dirFS(dir string) fs.FS {
	if dir == "" {
		dir = "."
	}
	return os.DirFS(dir)
}
