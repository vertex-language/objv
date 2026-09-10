package sysroot_test

import (
	"errors"
	"io/fs"
	"strings"
	"testing"

	"github.com/vertex-language/objv/sysroot"
)

// Every test here drives a fake Host, so the answers are the same on a Mac
// with three Xcodes, a Linux box with no GNUstep, and CI. The one thing a
// real machine is asked is whether the embedded headers are there, which is a
// fact about the binary rather than about the host.

// fakeHost is a machine described by three maps.
type fakeHost struct {
	env   map[string]string
	dirs  map[string]bool
	files map[string]string
	runs  map[string]string
}

func (h fakeHost) Getenv(k string) string { return h.env[k] }
func (h fakeHost) IsDir(p string) bool    { return h.dirs[p] }

func (h fakeHost) ReadFile(p string) (string, error) {
	if s, ok := h.files[p]; ok {
		return s, nil
	}
	return "", errors.New("no such file")
}

func (h fakeHost) Run(name string, args ...string) (string, error) {
	if s, ok := h.runs[name+" "+strings.Join(args, " ")]; ok {
		return s, nil
	}
	return "", errors.New("not found")
}

const sdkPath = "/SDKs/MacOSX.sdk"

// settings is the part of SDKSettings.json this package reads, with the
// values Xcode 26 actually carries.
const settings = `{
  "Version": "26.4",
  "SupportedTargets": {
    "macosx": {
      "MinimumDeploymentTarget": "10.13",
      "DefaultDeploymentTarget": "26.4",
      "Archs": ["x86_64", "x86_64h", "arm64", "arm64e"]
    }
  }
}`

func darwinHost() fakeHost {
	return fakeHost{
		env: map[string]string{},
		dirs: map[string]bool{
			sdkPath:                                   true,
			sdkPath + "/usr/include":                  true,
			sdkPath + "/usr/lib":                      true,
			sdkPath + "/System/Library/Frameworks":    true,
			sdkPath + "/System/Library/SubFrameworks": true,
			"/usr/local/lib":                          true,
		},
		files: map[string]string{sdkPath + "/SDKSettings.json": settings},
		runs: map[string]string{
			"xcrun --show-sdk-path":   sdkPath + "\n",
			"sw_vers -productVersion": "26.5.1\n",
		},
	}
}

func names(es []sysroot.Entry) []string {
	out := make([]string, 0, len(es))
	for _, e := range es {
		out = append(out, e.Name)
	}
	return out
}

func TestDarwinResolve(t *testing.T) {
	r := sysroot.ResolveWith(darwinHost(),
		sysroot.Options{Target: "aarch64-macos", Hosted: true})

	// The builtin headers first, then the SDK's. There is no
	// /usr/local/include: clang drops it once a sysroot is in force.
	if got, want := names(r.Include), []string{"<builtin>", sdkPath + "/usr/include"}; !eq(got, want) {
		t.Errorf("include = %v, want %v", got, want)
	}
	// SubFrameworks is not an exotic case: a framework header reaches its
	// siblings through it.
	if got, want := names(r.Frameworks), []string{
		sdkPath + "/System/Library/Frameworks",
		sdkPath + "/System/Library/SubFrameworks",
	}; !eq(got, want) {
		t.Errorf("frameworks = %v, want %v", got, want)
	}
	if got, want := r.LibraryDirs, []string{"/usr/local/lib", sdkPath + "/usr/lib"}; !eq(got, want) {
		t.Errorf("library dirs = %v, want %v", got, want)
	}
	// libSystem re-exports libobjc, which is why an Objective-C program
	// links with -lSystem and nothing else.
	if got, want := r.Libraries, []string{"System"}; !eq(got, want) {
		t.Errorf("libraries = %v, want %v", got, want)
	}
	if len(r.Notes) != 0 {
		t.Errorf("notes on a complete host: %v", r.Notes)
	}
}

func TestSDKSettings(t *testing.T) {
	r := sysroot.ResolveWith(darwinHost(),
		sysroot.Options{Target: "aarch64-macos", Hosted: true})
	if !r.SDK.Found() {
		t.Fatal("no SDK found")
	}
	if got := r.SDK.Version.String(); got != "26.4" {
		t.Errorf("SDK version = %s, want 26.4", got)
	}
	if got := r.SDK.MinimumDeployment.String(); got != "10.13" {
		t.Errorf("minimum = %s, want 10.13", got)
	}
	if len(r.SDK.Archs) != 4 {
		t.Errorf("archs = %v", r.SDK.Archs)
	}
}

// An SDK with no readable settings is still an SDK: the headers are there,
// and refusing one over a missing JSON file would turn a cosmetic gap into
// "no SDK found".
func TestSDKWithoutSettings(t *testing.T) {
	h := darwinHost()
	delete(h.files, sdkPath+"/SDKSettings.json")
	r := sysroot.ResolveWith(h, sysroot.Options{Target: "aarch64-macos", Hosted: true})
	if !r.SDK.Found() {
		t.Fatal("an SDK with no settings file was rejected")
	}
	if !r.SDK.Version.IsZero() {
		t.Errorf("version invented: %v", r.SDK.Version)
	}
	if len(r.Include) != 2 {
		t.Errorf("include = %v", names(r.Include))
	}
}

// The four-step SDK lookup, in the order the platform's own tools use it.
func TestSDKLookupOrder(t *testing.T) {
	h := darwinHost()
	h.dirs["/other.sdk"] = true
	h.dirs["/env.sdk"] = true

	// -isysroot beats everything.
	h.env["SDKROOT"] = "/env.sdk"
	r := sysroot.ResolveWith(h, sysroot.Options{
		Target: "aarch64-macos", Hosted: true, SDKPath: "/other.sdk"})
	if r.SDK.Path != "/other.sdk" {
		t.Errorf("isysroot ignored: %s", r.SDK.Path)
	}

	// $SDKROOT beats xcrun.
	r = sysroot.ResolveWith(h, sysroot.Options{Target: "aarch64-macos", Hosted: true})
	if r.SDK.Path != "/env.sdk" {
		t.Errorf("SDKROOT ignored: %s", r.SDK.Path)
	}

	// With neither, xcrun answers.
	delete(h.env, "SDKROOT")
	r = sysroot.ResolveWith(h, sysroot.Options{Target: "aarch64-macos", Hosted: true})
	if r.SDK.Path != sdkPath {
		t.Errorf("xcrun ignored: %s", r.SDK.Path)
	}

	// With none of the three, the Command Line Tools SDK at its fixed path.
	delete(h.runs, "xcrun --show-sdk-path")
	const clt = "/Library/Developer/CommandLineTools/SDKs/MacOSX.sdk"
	h.dirs[clt] = true
	r = sysroot.ResolveWith(h, sysroot.Options{Target: "aarch64-macos", Hosted: true})
	if r.SDK.Path != clt {
		t.Errorf("command line tools SDK ignored: %s", r.SDK.Path)
	}
}

// A host with no SDK still resolves — to the builtins — and says what is
// missing. The failure that matters is the #import that does not find its
// file, reported there.
func TestNoSDK(t *testing.T) {
	h := fakeHost{env: map[string]string{}, dirs: map[string]bool{}}
	r := sysroot.ResolveWith(h, sysroot.Options{Target: "aarch64-macos", Hosted: true})
	if got := names(r.Include); !eq(got, []string{"<builtin>"}) {
		t.Errorf("include = %v", got)
	}
	if len(r.Notes) == 0 || !strings.Contains(r.Notes[0], "no macOS SDK") {
		t.Errorf("notes = %v", r.Notes)
	}
}

// The deployment target: -mmacosx-version-min, then the environment, then
// the machine doing the building — clang's order, and clang's surprise that
// the last one is the host and not the SDK's default.
func TestDeploymentTarget(t *testing.T) {
	h := darwinHost()

	r := sysroot.ResolveWith(h, sysroot.Options{Target: "aarch64-macos", Hosted: true})
	if got := r.Deployment.String(); got != "26.0" {
		t.Errorf("default deployment = %s, want the host's major (26.0)", got)
	}

	h.env["MACOSX_DEPLOYMENT_TARGET"] = "13.2"
	r = sysroot.ResolveWith(h, sysroot.Options{Target: "aarch64-macos", Hosted: true})
	if got := r.Deployment.String(); got != "13.2" {
		t.Errorf("environment deployment = %s, want 13.2", got)
	}

	r = sysroot.ResolveWith(h, sysroot.Options{
		Target: "aarch64-macos", Hosted: true,
		Deployment: sysroot.Version{Major: 12, Minor: 3}})
	if got := r.Deployment.String(); got != "12.3" {
		t.Errorf("stated deployment = %s, want 12.3", got)
	}
}

// Apple Silicon did not exist before macOS 11, so an arm64 build asking for
// 10.13 is asking for something that cannot be. clang raises it silently;
// this raises it and says so.
func TestArchitectureFloor(t *testing.T) {
	h := darwinHost()
	old := sysroot.Version{Major: 10, Minor: 13}

	r := sysroot.ResolveWith(h, sysroot.Options{
		Target: "aarch64-macos", Hosted: true, Deployment: old})
	if got := r.Deployment.String(); got != "11.0" {
		t.Errorf("arm64 deployment = %s, want 11.0", got)
	}
	if len(r.Notes) == 0 {
		t.Error("raising the deployment target said nothing")
	}

	// x86_64 has no such floor.
	r = sysroot.ResolveWith(h, sysroot.Options{
		Target: "x86_64-macos", Hosted: true, Deployment: old})
	if got := r.Deployment.String(); got != "10.13" {
		t.Errorf("x86_64 deployment = %s, want 10.13", got)
	}
}

// --freestanding is the builtins and nothing else: a program that names no
// libc names its own libraries.
func TestFreestanding(t *testing.T) {
	r := sysroot.ResolveWith(darwinHost(),
		sysroot.Options{Target: "aarch64-macos", Hosted: false})
	if got := names(r.Include); !eq(got, []string{"<builtin>"}) {
		t.Errorf("include = %v", got)
	}
	if len(r.Frameworks) != 0 || len(r.LibraryDirs) != 0 || len(r.Libraries) != 0 {
		t.Error("a freestanding resolve produced platform entries")
	}
}

// OBJV_INCLUDE_PATH holds user directories, so its entries are not System: a
// warning in one is the user's to see every time.
func TestIncludePathEnvironment(t *testing.T) {
	h := darwinHost()
	h.env["OBJV_INCLUDE_PATH"] = "/a:/missing:/b"
	h.dirs["/a"] = true
	h.dirs["/b"] = true

	r := sysroot.ResolveWith(h, sysroot.Options{Target: "aarch64-macos", Hosted: true})
	if got := names(r.Include); !eq(got, []string{"/a", "/b", "<builtin>", sdkPath + "/usr/include"}) {
		t.Errorf("include = %v", got)
	}
	for _, e := range r.Include[:2] {
		if e.System {
			t.Errorf("%s was mounted as a system directory", e.Name)
		}
	}
}

// GNUstep: three libraries where Darwin needs one, because nothing there
// re-exports anything.
func TestGNUstep(t *testing.T) {
	h := fakeHost{
		env: map[string]string{"GNUSTEP_SYSTEM_ROOT": "/usr/GNUstep/System"},
		dirs: map[string]bool{
			"/usr/GNUstep/System":         true,
			"/usr/GNUstep/System/include": true,
			"/usr/GNUstep/System/lib":     true,
			"/usr/include":                true,
			"/usr/lib/x86_64-linux-gnu":   true,
		},
	}
	r := sysroot.ResolveWith(h, sysroot.Options{Target: "x86_64-linux", Hosted: true})
	if got := names(r.Include); !eq(got, []string{
		"<builtin>", "/usr/GNUstep/System/include", "/usr/include"}) {
		t.Errorf("include = %v", got)
	}
	if got, want := r.Libraries, []string{"objc", "gnustep-base"}; !eq(got, want) {
		t.Errorf("libraries = %v, want %v", got, want)
	}
	if r.SDK.Found() {
		t.Error("a GNUstep target reported an SDK")
	}
	// A framework is an Apple idea; no other platform has one.
	if len(r.Frameworks) != 0 {
		t.Errorf("frameworks on GNUstep: %v", names(r.Frameworks))
	}
}

// A target nothing is wired up for still resolves, and says so.
func TestUnknownTarget(t *testing.T) {
	r := sysroot.ResolveWith(darwinHost(),
		sysroot.Options{Target: "aarch64-fuchsia", Hosted: true})
	if len(r.Notes) == 0 || !strings.Contains(r.Notes[0], "aarch64-fuchsia") {
		t.Errorf("notes = %v", r.Notes)
	}
}

// The builtin headers are compiled into the binary; their absence is a build
// error of objv itself, not a runtime condition.
func TestBuiltinHeaders(t *testing.T) {
	want := []string{
		"float.h", "iso646.h", "limits.h", "stdalign.h", "stdarg.h",
		"stdatomic.h", "stdbool.h", "stddef.h", "stdint.h", "stdnoreturn.h",
	}
	for _, name := range want {
		if _, err := fs.Stat(sysroot.BuiltinFS(), name); err != nil {
			t.Errorf("missing builtin header %s: %v", name, err)
		}
	}
	// Nothing Objective-C: <objc/objc.h> belongs to the runtime and is
	// shipped by the SDK.
	if _, err := fs.Stat(sysroot.BuiltinFS(), "objc"); err == nil {
		t.Error("objv ships an Objective-C header of its own")
	}
}

func TestVersion(t *testing.T) {
	for _, c := range []struct {
		text  string
		want  sysroot.Version
		ok    bool
		macro int
	}{
		{"10.13", sysroot.Version{Major: 10, Minor: 13}, true, 101300},
		{"10.15.4", sysroot.Version{Major: 10, Minor: 15, Patch: 4}, true, 101504},
		{"11.0", sysroot.Version{Major: 11}, true, 110000},
		{"26.4", sysroot.Version{Major: 26, Minor: 4}, true, 260400},
		{"26", sysroot.Version{Major: 26}, true, 260000},
		// "26.4.99" is what an SDK's MaximumDeploymentTarget says.
		{"26.4.99", sysroot.Version{Major: 26, Minor: 4, Patch: 99}, true, 260499},
		{"", sysroot.Version{}, false, 0},
		{"beta", sysroot.Version{}, false, 0},
	} {
		got, ok := sysroot.ParseVersion(c.text)
		if ok != c.ok || got != c.want {
			t.Errorf("ParseVersion(%q) = %v, %v; want %v, %v", c.text, got, ok, c.want, c.ok)
			continue
		}
		if ok && got.MacroValue() != c.macro {
			t.Errorf("%q macro value = %d, want %d", c.text, got.MacroValue(), c.macro)
		}
	}

	if got := (sysroot.Version{Major: 13, Minor: 2}).Triple(); got != "13.2.0" {
		t.Errorf("Triple() = %s, want 13.2.0", got)
	}
	if !(sysroot.Version{Major: 10, Minor: 15}).Less(sysroot.Version{Major: 11}) {
		t.Error("10.15 does not precede 11.0")
	}
}

// The predefines are the platform's, and each one is load-bearing: every
// value was found by preprocessing <Foundation/Foundation.h> and reading what
// broke without it.
func TestPredefines(t *testing.T) {
	opt := sysroot.Options{Target: "aarch64-macos", Hosted: true}
	r := sysroot.ResolveWith(darwinHost(), opt)
	got := map[string]bool{}
	for _, d := range sysroot.Predefines(opt, r) {
		got[d] = true
	}
	for _, want := range []string{
		"__APPLE__=1", "__MACH__=1",
		// TargetConditionals.h tests this beside __GNUC__, and #errors
		// "unknown compiler" without it.
		"__APPLE_CC__=6000",
		// CFBase.h and NSByteOrder.h both #error without this one.
		"__LITTLE_ENDIAN__=1",
		"__BYTE_ORDER__=__ORDER_LITTLE_ENDIAN__",
		// CarbonCore's fp.h #errors on a CPU it does not recognize.
		"__aarch64__=1", "__arm64__=1", "__LP64__=1",
		// What every availability macro in every Cocoa header compares
		// against, and therefore what decides whether a method is
		// declared at all. The host is 26.5.1, so the target is 26.0.
		"__ENVIRONMENT_MAC_OS_X_VERSION_MIN_REQUIRED__=260000",
	} {
		if !got[want] {
			t.Errorf("missing predefine %s", want)
		}
	}

	// x86_64 answers to four names, because different headers picked
	// different ones and all of them are still in use.
	r64 := sysroot.ResolveWith(darwinHost(),
		sysroot.Options{Target: "x86_64-macos", Hosted: true})
	got = map[string]bool{}
	for _, d := range sysroot.Predefines(sysroot.Options{Target: "x86_64-macos", Hosted: true}, r64) {
		got[d] = true
	}
	for _, want := range []string{"__x86_64__=1", "__amd64__=1", "__LP64__=1"} {
		if !got[want] {
			t.Errorf("missing predefine %s", want)
		}
	}
	if got["__aarch64__=1"] {
		t.Error("an x86_64 target got the aarch64 macros")
	}
}

func TestTargetSplit(t *testing.T) {
	for _, c := range []struct{ target, arch, os string }{
		{"aarch64-macos", "aarch64", "macos"},
		{"x86_64-linux", "x86_64", "linux"},
		{"x86_64-windows", "x86_64", "windows"},
		{"aarch64", "aarch64", ""},
	} {
		if got := sysroot.ArchOf(c.target); got != c.arch {
			t.Errorf("ArchOf(%q) = %q, want %q", c.target, got, c.arch)
		}
		if got := sysroot.OSOf(c.target); got != c.os {
			t.Errorf("OSOf(%q) = %q, want %q", c.target, got, c.os)
		}
	}
}

func eq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
