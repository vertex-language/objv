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

// A Compiler configures and drives compilation for a target.
//
// Target configuration (predefined macros, include/framework search paths, SDK,
// and deployment target) is resolved once on first use and reused across inputs.
// A Compiler must not be mutated after its first use.
type Compiler struct {
	// Target names the target triple (e.g. "aarch64-macos"); empty defaults to host.
	Target string

	// IncludeDirs are searched for headers before sysroot include paths.
	IncludeDirs []string

	// FrameworkDirs are searched for frameworks (-F) before SDK paths.
	FrameworkDirs []string

	// Defines are -D and -U macro definitions in CLI order.
	Defines []preprocessor.Predefine

	// PreIncludes are header files included before the main source.
	PreIncludes []string

	// SDKPath overrides SDK discovery (-isysroot).
	SDKPath string

	// Deployment overrides the minimum OS deployment target version.
	Deployment sysroot.Version

	// ARC enables automatic reference counting (-fobjc-arc).
	ARC bool

	// Freestanding compiles against builtin headers only, without SDK or system libraries.
	Freestanding bool

	// KeepComments retains COMMENT tokens through preprocessing.
	KeepComments bool

	// PP controls whether preprocessing runs (PPAuto, PPAlways, PPNever).
	PP Tristate

	// SourceDate overrides the timestamp for __DATE__ and __TIME__.
	SourceDate *time.Time

	// Host provides the filesystem and environment interface for SDK discovery.
	Host sysroot.Host

	// OnDiagnostic receives diagnostics as they are produced.
	OnDiagnostic func(Diagnostic)

	// Producer is the toolchain identifier string stamped into output files.
	Producer string

	once  sync.Once
	cfg   preprocessor.Config
	res   sysroot.Result
	cfgTt Target
	cfgEr error
}

// An Env is the resolved build configuration: search paths, target settings,
// and predefined macros.
type Env struct {
	Target     Target
	Hosted     bool
	ARC        bool
	SDK        sysroot.SDK
	Deployment sysroot.Version

	Search     []preprocessor.Mount
	Frameworks []preprocessor.Mount
	Predefines []preprocessor.Predefine

	// LibraryDirs and Libraries are library search paths and default libraries.
	LibraryDirs []string
	Libraries   []string

	// Notes contains non-fatal informational messages from sysroot resolution.
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

// target resolves the target name into a Target.
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

// config resolves target, sysroot, and preprocessor options once.
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
		cfg.Hosted = !c.Freestanding

		// IncludeDirs and FrameworkDirs precede sysroot paths.
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
		cfg.Feature = HasFeatureFunc(c.ARC)
		cfg.Extension = HasExtensionFunc(c.ARC)
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
