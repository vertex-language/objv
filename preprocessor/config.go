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

// Mount is one entry in a search list: a filesystem and the name it is known
// by.
//
// The preprocessor reads fs.FS and never os. Where headers live is sysroot's
// question, answered once in the objv package and handed here as data — which
// is what makes phase 4 testable against fstest.MapFS with no host and no SDK
// involved, and what keeps `objv env` able to print the resolved lists before
// the build runs.
//
// Name is what a path resolved against this mount is spelled as in __FILE__,
// in diagnostics and in dependency output. It is the directory as given,
// never made absolute.
type Mount struct {
	Name string
	FS   fs.FS

	// System marks a mount whose headers are not the user's code: warnings
	// sited inside one are reported once per header rather than once per
	// inclusion. Every framework in an SDK is one, and so is every platform
	// include directory — which matters more here than in a C compiler,
	// because #import <Foundation/Foundation.h> reaches some nine hundred
	// headers and a warning repeated per inclusion would bury the program.
	System bool
}

// A Triple is a target named the way a target triple names one:
// arch-vendor-os-environment. It is what __is_target_arch and its three
// neighbours compare against.
//
// Every component is optional and an empty one matches nothing, which is the
// honest answer where the target model has no opinion. The environment is the
// component Darwin actually leans on: TargetConditionals.h decides
// TARGET_OS_MACCATALYST and TARGET_OS_SIMULATOR from it, and those two decide
// what half of UIKit declares.
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

// Predefine is one -D or -U, or one macro the target model contributes.
//
// Text is the spelling after the flag: "DEBUG", "VERSION=3", "MAX(a,b)=..."
// for a define, and a bare name for an undef. It is parsed by the same
// #define grammar directive.go runs, so -D and #define cannot drift apart.
//
// Target-dependent macros (__CHAR_BIT__, __SIZEOF_LONG__, __INT_MAX__,
// __APPLE__, __MACH__ and kin) arrive through this list too. They are facts
// about a target model, and preprocessor does not import one — the objv
// package computes them and puts them here, the same inversion that keeps
// sysroot out of phase 4.
type Predefine struct {
	Kind PredefineKind
	Text string
}

// Config is everything phase 4 needs from its caller. The preprocessor is a
// pure function of it: same Config, same input, same output, always.
type Config struct {
	// Search is the include list, in the order §6.10.2 walks it: -I
	// directories first, then objv's builtin headers, then the platform's.
	// There is no second list — #include "..." looks in the including file's
	// directory first and then walks this one, and #include <...> skips that
	// first step.
	Search []Mount

	// Frameworks is the framework search list: -F directories, then the
	// SDK's. A framework include is an angled include whose first path
	// component names a framework, and it is what almost every line of
	// Objective-C starts with:
	//
	//	#import <Foundation/NSString.h>
	//	          ^^^^^^^^^^ ^^^^^^^^^^
	//	          framework  header inside it
	//
	// resolving to Foundation.framework/Headers/NSString.h in the first
	// framework directory that has it, and to PrivateHeaders/NSString.h
	// after that. The two lists are searched in order — Search first, then
	// Frameworks — so a plain directory holding a real Foundation/ wins, as
	// it does in clang.
	Frameworks []Mount

	// Source is the directory the primary input was read from, as a mount.
	//
	// §6.10.2p3 has a quoted #include look beside the file that wrote it, and
	// the primary source file is a file like any other: `#import "Cache.h"`
	// at the top of src/Cache.m must find src/Cache.h without a -I. Every
	// header reached from there gets this for free — its Origin carries the
	// mount it was found in — but the primary file has no Origin until one is
	// made, which is what this supplies.
	Source Mount

	// Predefines are applied in order before the primary source file is read.
	Predefines []Predefine

	// Triple is what the __is_target_* operators answer from.
	Triple Triple

	// Builtin, Feature, Extension and Attribute answer clang's four
	// interrogation operators. Each is a function rather than a list for the
	// same reason: the answer is not phase 4's to give.
	//
	// What ARC does, which attributes the analyzer honours, which language
	// features this compilation has — those are decided where the front end
	// implements them and where the command line turns them on, and a copy
	// of that rule here would be a second rule that could disagree with the
	// first. Nil answers no, which is the right answer for a caller that has
	// not said: a header is entitled to be told no and take its fallback.
	//
	// Nothing about Objective-C is optional to this package, but nearly
	// everything about it is optional to a Cocoa header, and this is the
	// only way one asks:
	//
	//	#if __has_feature(objc_arc)
	//	#if __has_attribute(objc_designated_initializer)
	//	#if __has_feature(objc_generics)
	//
	// Extension falls back to Feature when nil, which is clang's rule:
	// __has_extension is true wherever __has_feature is, and true besides
	// where the feature exists only as an extension to the dialect in force.
	Builtin   func(name string) bool
	Feature   func(name string) bool
	Extension func(name string) bool
	Attribute func(name string) bool

	// PreIncludes are processed before the main input, in order, exactly as
	// if #include'd at the top of it. This is --include / -include.
	PreIncludes []string

	// Std selects __STDC_VERSION__ and the standard a diagnostic cites.
	Std Std

	// Hosted sets __STDC_HOSTED__.
	Hosted bool

	// Epoch clamps __DATE__ and __TIME__. Nil means the caller supplied no
	// SOURCE_DATE_EPOCH; determinism then depends on the caller, so the CLI
	// always supplies one. Stored as a value, never read from the clock here.
	Epoch *time.Time

	// MaxIncludeDepth caps nesting. §5.2.4.1 guarantees 15; the limit exists
	// to turn a cyclic include into one diagnostic instead of a stack
	// overflow, so it is generous — an SDK umbrella header nests deeply on
	// purpose.
	MaxIncludeDepth int

	// MaxExpansionDepth caps nested expansion. Prosser's hide sets already
	// guarantee termination for well-formed input; this catches the input
	// that is not, and the combinatorial blowups that terminate but not in
	// this decade.
	MaxExpansionDepth int

	// TrackDeps records every file #include and #import reached, for --deps.
	TrackDeps bool

	// KeepComments retains COMMENT tokens in the output. --emit mi does not
	// want them dropped silently; the parser never sees them.
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
