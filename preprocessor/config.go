package preprocessor

import (
	"io/fs"
	"time"
)

// Std is the C substrate the preprocessor predefines for. It reaches phase 4
// because __STDC_VERSION__ is phase 4's to define, and because a diagnostic
// that cites chapter and verse needs to know which document it is citing.
// Objective-C itself has no version macro of this shape; __OBJC__ and
// __OBJC2__ say what they have to say and neither carries a number.
type Std uint8

const (
	C11 Std = iota
	C17
)

// Version returns the __STDC_VERSION__ value for the standard.
func (s Std) Version() string {
	if s == C11 {
		return "201112L"
	}
	return "201710L"
}

func (s Std) String() string {
	if s == C11 {
		return "c11"
	}
	return "c17"
}

// Mount is one entry in a header search list: a filesystem and its display name.
type Mount struct {
	Name string
	FS   fs.FS

	// System marks a mount whose headers are system headers (warnings reported once per header).
	System bool
}

// Triple describes a target triple (arch-vendor-os-environment) used by __is_target_* operators.
type Triple struct {
	Arch        string
	Vendor      string
	OS          string
	Environment string
}

// PredefineKind distinguishes the two command-line operations.
type PredefineKind uint8

const (
	PredefineDefine PredefineKind = iota
	PredefineUndef
)

// Predefine represents a -D or -U command-line option or target-provided macro definition.
type Predefine struct {
	Kind PredefineKind
	Text string
}

// Config provides options and hooks for the preprocessor.
type Config struct {
	// Search is the include path list (-I directories, builtins, and SDK headers).
	Search []Mount

	// Frameworks is the framework search list (-F directories and SDK frameworks).
	Frameworks []Mount

	// Source is the mount for the directory of the primary input file.
	Source Mount

	// Predefines are macros defined or undefined before reading source.
	Predefines []Predefine

	// Triple is used by __is_target_* operators.
	Triple Triple

	// Builtin, Feature, Extension and Attribute answer clang's interrogation
	// operators (__has_builtin, __has_feature, __has_extension, __has_attribute).
	Builtin   func(name string) bool
	Feature   func(name string) bool
	Extension func(name string) bool
	Attribute func(name string) bool

	// PreIncludes are header files included before the main source.
	PreIncludes []string

	// Std selects the C language standard (__STDC_VERSION__).
	Std Std

	// Hosted sets __STDC_HOSTED__.
	Hosted bool

	// Epoch overrides the timestamp for __DATE__ and __TIME__.
	Epoch *time.Time

	// MaxIncludeDepth caps nested inclusion depth.
	MaxIncludeDepth int

	// MaxExpansionDepth caps nested macro expansion depth.
	MaxExpansionDepth int

	// TrackDeps records header dependencies.
	TrackDeps bool

	// KeepComments retains COMMENT tokens in the output.
	KeepComments bool
}

// Default fills in the limits a caller left zero. It does not invent a search
// list, predefines or an epoch: those are the caller's to supply, and a zero
// Config preprocesses a self-contained file with no headers and no macros,
// which is exactly what a test wants.
func (c Config) Default() Config {
	if c.MaxIncludeDepth == 0 {
		c.MaxIncludeDepth = 200
	}
	if c.MaxExpansionDepth == 0 {
		c.MaxExpansionDepth = 4000
	}
	return c
}

// Now returns the time __DATE__ and __TIME__ expand against: the configured
// epoch, or the zero time when none was supplied. Nothing in this package
// reads a clock, so a build with no epoch is still deterministic per run and
// the CLI is the only place that could make it otherwise.
func (c Config) Now() time.Time {
	if c.Epoch != nil {
		return c.Epoch.UTC()
	}
	return time.Time{}
}

// has answers one of the four interrogation operators through the hook the
// caller supplied. A nil hook answers no.
func (c Config) has(hook func(string) bool, name string) bool {
	return hook != nil && hook(name)
}
