// Package sysroot resolves target headers, frameworks, libraries, SDKs,
// and deployment targets for compilation and linking.
package sysroot

import (
	"io/fs"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// Entry represents one resolved include or framework directory.
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

// Result holds all resolved paths and configuration.
type Result struct {
	Include     []Entry    // include search paths in lookup order
	Frameworks  []Entry    // framework search paths
	LibraryDirs []string   // library search paths (-L)
	Libraries   []string   // default runtime libraries (-l)
	SDK         SDK        // resolved platform SDK
	Deployment  Version    // target OS deployment version
	Notes       []string   // informational notes or diagnostic hints
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

// dirEntries probes directories in order and mounts existing ones as system headers.
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
