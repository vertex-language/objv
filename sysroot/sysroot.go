// Package sysroot answers the question phase 4 cannot: where does this host
// keep the target's headers, frameworks and libraries?
//
// Resolve produces everything a hosted compilation needs from the machine —
// the include list, the framework list, the library directories, the SDK it
// found and the deployment target it settled on. The result is data: the objv
// package turns it into preprocessor.Config and linker arguments, and
// `objv env` prints it before the build runs.
//
// This package imports the standard library only, and is imported by the objv
// package alone. The preprocessor never learns what an SDK is; this package
// never learns what a token is.
//
// # Why an Objective-C compiler needs more of this than a C compiler
//
// A C program can be compiled against no system headers at all. An
// Objective-C one effectively cannot: the language's own literals are sends
// to Foundation classes, and the first line of almost every file is
//
//	#import <Foundation/Foundation.h>
//
// which is not a directory lookup but a framework lookup — Foundation.framework
// /Headers/Foundation.h — and which reaches some nine hundred headers whose
// every declaration is gated on a deployment target this package has to
// determine. That is why Resolve returns a Deployment and an SDK version
// where a C compiler's would return only paths.
package sysroot

import (
	"io/fs"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// Entry is one resolved include or framework directory: a filesystem, the
// name a path resolved against it is known by, and whether its headers are
// the system's rather than the user's.
//
// It mirrors preprocessor.Mount field for field, deliberately, but this
// package does not import preprocessor — the conversion is one loop in the
// objv package, and it keeps the dependency arrow pointing the right way:
// sysroot is below the CLI, beside nothing.
type Entry struct {
	Name   string
	FS     fs.FS
	System bool
}

// Host is everything Resolve reads from the machine: environment variables,
// directory existence, one small file, and one tool's output.
//
// The indirection exists because probing is impure by nature but the ordering
// logic is not. With Host injected, resolve is a pure function a test can
// drive as a Mac with no Xcode, a Linux box with no GNUstep, or a machine
// whose SDK is three releases old — on any machine.
type Host interface {
	// Getenv returns the named variable, "" when unset.
	Getenv(key string) string
	// IsDir reports whether path exists and is a directory.
	IsDir(path string) bool
	// ReadFile returns the contents of a small file.
	ReadFile(path string) (string, error)
	// Run executes a tool and returns its standard output.
	Run(name string, args ...string) (string, error)
}

// osHost is the real machine.
type osHost struct{}

func (osHost) Getenv(key string) string { return os.Getenv(key) }

func (osHost) IsDir(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.IsDir()
}

func (osHost) ReadFile(path string) (string, error) {
	b, err := os.ReadFile(path)
	return string(b), err
}

func (osHost) Run(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).Output()
	return string(out), err
}

// Options is what the command line tells Resolve.
type Options struct {
	// Target is the target name: "aarch64-macos", "x86_64-linux".
	Target string

	// Hosted is false for --freestanding, which is how a caller says it
	// wants the builtin headers and nothing else.
	Hosted bool

	// SDKPath overrides SDK discovery outright, as clang's -isysroot does.
	// An empty string means "find one".
	SDKPath string

	// Deployment overrides the deployment target, as
	// -mmacosx-version-min does. The zero Version means "decide one".
	Deployment Version
}

// Result is everything Resolve found.
type Result struct {
	// Include is the include list, in the order §6.10.2 walks it: objv's
	// builtin headers first, then the platform's. -I directories precede
	// all of it and are the CLI's to prepend.
	Include []Entry

	// Frameworks is the framework search list. -F directories precede it,
	// again from the CLI.
	Frameworks []Entry

	// LibraryDirs is where a -l name is looked for, in order.
	LibraryDirs []string

	// Libraries is the runtime a hosted link gets when the caller named
	// none. On Darwin that is one name and on GNUstep it is three; see
	// library.go for why the counts differ.
	Libraries []string

	// SDK is what was found, and is the zero SDK on a platform that has
	// none. Its Path is what a Mach-O link passes as -syslibroot: the
	// headers and the stub libraries have to come from one SDK, and
	// resolving it twice by two routes is how a machine ends up compiling
	// against one and failing to link against another.
	SDK SDK

	// Deployment is the oldest OS this build will run on, which decides
	// what half of every Cocoa header declares. It is the zero Version
	// where the platform has no such notion.
	Deployment Version

	// Notes are worth surfacing when something expected was not found.
	// They are advice for `objv env` and for diagnostics, never errors: a
	// host with no SDK still resolves — to the builtins — and the failure
	// that matters is the #import that does not find its file, reported
	// there, with this list to point at.
	Notes []string
}

// Resolve reads this machine.
func Resolve(opt Options) Result { return ResolveWith(nil, opt) }

// ResolveWith is Resolve with the host injected: a nil Host is the real
// machine, and any other is asked instead of it. A caller that supplies one
// resolves headers without reading this machine's environment or filesystem
// at all, which is what makes a hermetic build hermetic.
func ResolveWith(h Host, opt Options) Result {
	if h == nil {
		h = osHost{}
	}
	return resolve(h, runtime.GOOS, opt)
}

// resolve is Resolve with the impurities injected: the host to probe and the
// OS to probe it as. Tests call this; nothing else should.
func resolve(h Host, goos string, opt Options) Result {
	var r Result

	// OBJV_INCLUDE_PATH holds user directories, so its entries are not
	// System: a warning in one is the user's to see every time.
	for _, dir := range splitList(h.Getenv("OBJV_INCLUDE_PATH"), goos) {
		if h.IsDir(dir) {
			r.Include = append(r.Include, Entry{Name: dir, FS: os.DirFS(dir)})
		}
	}
	r.Include = append(r.Include, builtinEntry())

	if !opt.Hosted {
		// --freestanding is that step alone: the headers ISO requires of a
		// freestanding implementation are exactly the ones objv carries in
		// the binary. A program that names no libc names its own
		// libraries, with -L and -l.
		return r
	}

	switch osOf(opt.Target) {
	case "macos":
		darwinResolve(h, opt, &r)
	case "linux", "windows":
		gnustepResolve(h, opt, &r)
	default:
		r.Notes = append(r.Notes,
			"no system headers are wired up for "+opt.Target+"; use -I and -F")
	}
	return r
}

// dirEntries probes a list of directories in order and mounts the ones that
// exist. Everything the platform supplies is System: its headers are not the
// user's code, so a warning sited in one is reported once per header rather
// than once per inclusion — which matters far more here than in a C compiler,
// because one #import reaches hundreds of them.
func dirEntries(h Host, dirs []string) []Entry {
	var out []Entry
	for _, dir := range dirs {
		if h.IsDir(dir) {
			out = append(out, Entry{Name: dir, FS: os.DirFS(dir), System: true})
		}
	}
	return out
}

// existing filters a list of directories down to the ones that are there.
func existing(h Host, dirs []string) []string {
	var out []string
	for _, dir := range dirs {
		if h.IsDir(dir) {
			out = append(out, dir)
		}
	}
	return out
}

// splitList splits a PATH-shaped variable with the separator of the OS being
// resolved for — a parameter, not runtime.GOOS, so a test can exercise the
// Windows split on any machine.
func splitList(s, goos string) []string {
	if s == "" {
		return nil
	}
	sep := ":"
	if goos == "windows" {
		sep = ";"
	}
	var out []string
	for _, d := range strings.Split(s, sep) {
		if d = strings.TrimSpace(d); d != "" {
			out = append(out, d)
		}
	}
	return out
}

// ArchOf and OSOf split a target name. The vocabulary is the README's —
// "aarch64-macos", "x86_64-linux" — and the two halves are what the platform
// rules key on.
func ArchOf(target string) string { return archOf(target) }
func OSOf(target string) string   { return osOf(target) }

func archOf(target string) string {
	if i := strings.IndexByte(target, '-'); i >= 0 {
		return target[:i]
	}
	return target
}

func osOf(target string) string {
	if i := strings.IndexByte(target, '-'); i >= 0 {
		return target[i+1:]
	}
	return ""
}
