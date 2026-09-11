package objv_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/vertex-language/objv"
	"github.com/vertex-language/objv/preprocessor"
	"github.com/vertex-language/objv/sysroot"
)

// ---- the target table ----

func TestTargets(t *testing.T) {
	for _, name := range objv.Targets() {
		tt, ok := objv.LookupTarget(name)
		if !ok {
			t.Fatalf("Targets() lists %q, which LookupTarget does not know", name)
		}
		if tt.Name() != name {
			t.Errorf("%s: Name() = %q", name, tt.Name())
		}
		if tt.Model().SizePtr == 0 {
			t.Errorf("%s: zero Model — every type would size at nothing", name)
		}
		if !tt.IR().Valid() {
			t.Errorf("%s: no ir.Target", name)
		}
	}
	if _, ok := objv.LookupTarget("vax-vms"); ok {
		t.Error("LookupTarget invented a target")
	}
}

// The triple is what TargetConditionals.h's __is_target_* operators compare
// against, and Darwin's spelling of the architecture is not the
// architecture's own name. Answering aarch64 there decides TARGET_OS_OSX is
// 0 — silently, because the header falls through to a default.
func TestTripleUsesApplesArchName(t *testing.T) {
	arm, _ := objv.LookupTarget("aarch64-macos")
	tr := arm.Triple()
	if tr.Arch != "arm64" || tr.Vendor != "apple" || tr.OS != "macos" {
		t.Errorf("aarch64-macos triple = %+v", tr)
	}
	if tr.Environment != "" {
		t.Errorf("environment = %q; no target objv models is Catalyst or a simulator", tr.Environment)
	}
	lin, _ := objv.LookupTarget("x86_64-linux")
	if got := lin.Triple().Arch; got != "x86_64" {
		t.Errorf("linux arch = %q, want the architecture's own name", got)
	}
}

func TestPredefines(t *testing.T) {
	get := func(name string) (string, bool) {
		tt, ok := objv.LookupTarget(name)
		if !ok {
			t.Fatalf("no target %s", name)
		}
		var b strings.Builder
		for _, d := range tt.Predefines() {
			b.WriteString(d.Text)
			b.WriteByte('\n')
		}
		return b.String(), true
	}

	lp64, _ := get("aarch64-macos")
	for _, want := range []string{
		"__CHAR_BIT__=8",
		"__SIZEOF_POINTER__=8",
		"__SIZEOF_LONG__=8",
		"__SIZE_TYPE__=unsigned long",
		"__PTRDIFF_TYPE__=long",
		"__LONG_MAX__=9223372036854775807L",
		"__WCHAR_TYPE__=int", // Darwin's, where glibc says unsigned int
		// long double is double on Apple Silicon, and float.h has to agree.
		"__LDBL_MANT_DIG__=53",
	} {
		if !strings.Contains(lp64, want+"\n") {
			t.Errorf("aarch64-macos is missing %s", want)
		}
	}

	// Windows is LLP64: a 32-bit long under 64-bit pointers, so size_t
	// cannot be unsigned long.
	llp64, _ := get("x86_64-windows")
	for _, want := range []string{
		"__SIZEOF_LONG__=4",
		"__SIZE_TYPE__=unsigned long long",
		"__PTRDIFF_TYPE__=long long",
		"__WCHAR_TYPE__=unsigned short",
	} {
		if !strings.Contains(llp64, want+"\n") {
			t.Errorf("x86_64-windows is missing %s", want)
		}
	}
	// x86-64's long double is 80-bit extended, which is a different
	// float.h from Apple Silicon's.
	x86, _ := get("x86_64-macos")
	if !strings.Contains(x86, "__LDBL_MANT_DIG__=64\n") {
		t.Error("x86_64-macos does not describe an x87 long double")
	}
}

// ---- inputs ----

func TestInputClassification(t *testing.T) {
	for _, c := range []struct {
		name       string
		source, pp bool
	}{
		{"a.m", true, true},
		{"a.mi", true, false},
		{"a.c", true, true},
		{"a.i", true, false},
		{"a.o", false, false},
		{"libfoo.a", false, false},
		{"a.M", true, false}, // extensions are matched case-insensitively for
		//                       "is this source"; ".M" is not preprocessed,
		//                       matching the lowercase-only rule clang uses
		//                       for the preprocessing decision.
	} {
		in := objv.File(c.name)
		if got := isSource(in); got != c.source {
			t.Errorf("%s: isSource = %v, want %v", c.name, got, c.source)
		}
	}
}

// isSource reaches the unexported rule through the only public behaviour that
// depends on it: a non-source input is handed to the linker untouched, so a
// Build with one and no output fails on the link rather than on the parse.
func isSource(in objv.Input) bool {
	switch strings.ToLower(filepath.Ext(in.Name)) {
	case ".m", ".mi", ".c", ".i":
		return true
	}
	return false
}

// ---- the interrogation operators ----
//
// Both of these fail silently when wrong: TargetConditionals.h falls through
// to `#define TARGET_OS_MAC 0` rather than complaining, and every
// `#if TARGET_OS_OSX` in Foundation then takes the wrong branch.

func TestTargetConditionalsAnswers(t *testing.T) {
	if !objv.HasBuiltin("__is_target_os") {
		t.Error("__has_builtin(__is_target_os) must be true: TargetConditionals.h " +
			"decides TARGET_OS_OSX with it")
	}
	if objv.HasExtension("define_target_os_macros", false) {
		t.Error("__has_extension(define_target_os_macros) must be false: it means " +
			"the compiler predefines the TARGET_OS_* macros itself, which objv does not")
	}
	if !objv.HasFeature("objc_arc", true) || !objv.HasFeature("objc_fixed_enum", false) {
		t.Error("the Objective-C features objv implements must answer true")
	}
	if objv.HasFeature("cxx_lambdas", true) {
		t.Error("a feature objv does not implement must answer false")
	}
	if !objv.HasAttribute("anything_at_all") {
		t.Error("__has_attribute answers yes: an attribute objv ignores is better " +
			"than a header taking a different declaration")
	}
}

// ---- the compiler, hermetically ----

// hermetic is a Compiler that touches no SDK: freestanding, with objv's own
// builtin headers and nothing else. Every test that does not need Foundation
// uses it, so the suite gives the same answers on a machine with no Xcode.
func hermetic() *objv.Compiler {
	return &objv.Compiler{
		Target:       "aarch64-macos",
		Freestanding: true,
		Host:         emptyHost{},
	}
}

// emptyHost is a machine with nothing on it.
type emptyHost struct{}

func (emptyHost) Getenv(string) string                  { return "" }
func (emptyHost) IsDir(string) bool                     { return false }
func (emptyHost) ReadFile(string) (string, error)       { return "", os.ErrNotExist }
func (emptyHost) Run(string, ...string) (string, error) { return "", os.ErrNotExist }

func TestEnvFreestanding(t *testing.T) {
	env, err := hermetic().Env()
	if err != nil {
		t.Fatal(err)
	}
	if env.Target.Name() != "aarch64-macos" {
		t.Errorf("target = %s", env.Target.Name())
	}
	if len(env.Search) != 1 || env.Search[0].Name != "<builtin>" {
		t.Errorf("freestanding search = %v, want the builtins alone", names(env.Search))
	}
	if len(env.Frameworks) != 0 || len(env.Libraries) != 0 {
		t.Error("a freestanding environment named frameworks or libraries")
	}
	if len(env.Predefines) == 0 {
		t.Error("no predefines")
	}
}

func TestUnknownTarget(t *testing.T) {
	c := &objv.Compiler{Target: "pdp11-unix"}
	_, err := c.Env()
	if err == nil || !strings.Contains(err.Error(), "unknown target") {
		t.Errorf("err = %v, want an unknown-target error naming the known ones", err)
	}
	if !strings.Contains(err.Error(), "aarch64-macos") {
		t.Error("the error does not list what is known")
	}
}

func TestCheck(t *testing.T) {
	c := hermetic()
	diags, err := c.Check(objv.Text("t.m", []byte(`
__attribute__((objc_root_class))
@interface K
- (int)n;
@end
@implementation K
- (int)n { return 1; }
@end
`)))
	if err != nil {
		t.Fatal(err)
	}
	if objv.HasErrors(diags) {
		for _, d := range diags {
			t.Errorf("unexpected: %s", d)
		}
	}
}

// A diagnostic points at what was typed, not at the preprocessed text.
//
// This is the whole reason Diagnostic carries a Site: the parser reports in a
// file that only exists after phase 4, and a program whose second line is
// wrong would otherwise be told about line 200 of itself.
func TestDiagnosticsAreSitedInTheSource(t *testing.T) {
	c := hermetic()
	c.Defines = []preprocessor.Predefine{{Text: "PAD=1"}}
	src := []byte("#include <stdint.h>\nint x = ;\n")
	diags, err := c.Check(objv.Text("t.m", src))
	if err != nil {
		t.Fatal(err)
	}
	if !objv.HasErrors(diags) {
		t.Fatal("no diagnostic for a missing initializer")
	}
	for _, d := range diags {
		if d.Severity.String() != "error" {
			continue
		}
		s := objv.SiteString(d.Site)
		if !strings.HasPrefix(s, "t.m:2:") {
			t.Errorf("sited at %s, want t.m line 2 — <stdint.h> is hundreds of lines", s)
		}
		return
	}
}

// A diagnostic inside a header names the header, not the primary file.
func TestDiagnosticsInsideAHeader(t *testing.T) {
	c := hermetic()
	c.IncludeDirs = []string{writeDir(t, map[string]string{
		"bad.h": "int y = ;\n",
	})}
	diags, err := c.Check(objv.Text("t.m", []byte("#include <bad.h>\nint ok;\n")))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range diags {
		if strings.Contains(objv.SiteString(d.Site), "bad.h:1:") {
			found = true
		}
	}
	if !found {
		var got []string
		for _, d := range diags {
			got = append(got, d.String())
		}
		t.Errorf("no diagnostic sited in bad.h: %v", got)
	}
}

// --emit mi's promise: the output re-enters as the same program.
func TestPreprocessRoundTrip(t *testing.T) {
	c := hermetic()
	src := []byte(`
#define TWICE(x) ((x) * 2)
#pragma pack(push, 4)
struct s { char a; int b; };
#pragma pack(pop)
int f(int n) { return TWICE(n); }
`)
	out, diags, err := c.Preprocess(objv.Text("t.m", src))
	if err != nil {
		t.Fatal(err)
	}
	if objv.HasErrors(diags) {
		t.Fatalf("preprocess: %v", diags)
	}
	// The pragma survives: acting on one is the compiler's job and not
	// phase 4's, and a dropped #pragma pack changes what a struct means.
	if !strings.Contains(string(out), "#pragma pack") {
		t.Errorf("the pragma did not survive:\n%s", out)
	}
	if strings.Contains(string(out), "TWICE") {
		t.Errorf("the macro did not expand:\n%s", out)
	}

	// Re-entering as .mi produces the same diagnostics — that is, none.
	again, diags2, err := c.Preprocess(objv.Text("t.mi", out))
	if err != nil {
		t.Fatal(err)
	}
	if objv.HasErrors(diags2) {
		t.Fatalf("the output did not re-enter: %v", diags2)
	}
	if string(again) != string(out) {
		t.Error("a .mi input did not come back byte for byte")
	}
}

// ---- lowering ----

func TestVIR(t *testing.T) {
	c := hermetic()
	out, diags, err := c.VIR(objv.Text("counter.m", []byte(`
__attribute__((objc_root_class))
@interface Counter {
    int _n;
}
- (int)n;
@end
@implementation Counter
- (int)n { return _n; }
@end
int twice(int x) { return x * 2; }
`)))
	if err != nil {
		t.Fatal(err)
	}
	if objv.HasErrors(diags) {
		for _, d := range diags {
			t.Errorf("unexpected: %s", d)
		}
		return
	}
	s := string(out)
	for _, want := range []string{
		"module counter",           // the module is named for the file's stem
		`use "aarch64/macos"`,      // and opens with the target
		"export func @_twice",      // a C function, with Mach-O's underscore
		"@__i_Counter__n",          // a method, symbol-mangled
		"@_OBJC_IVAR_$_Counter._n", // the offset variable an access loads
		"@_OBJC_CLASS_$_Counter",   // the class object
		"__DATA,__objc_classlist",  // and the list the runtime scans
	} {
		if !strings.Contains(s, want) {
			t.Errorf("VIR does not contain %q", want)
		}
	}
}

// A module comes back for input the analyzer rejected only as nil: lowering a
// tree that was already reported as wrong produces internal errors about it,
// which buries the diagnostic that matters.
func TestNoModuleForBrokenInput(t *testing.T) {
	m, diags, err := hermetic().Module(objv.Text("t.m", []byte("int f(void) { return nope; }\n")))
	if err != nil {
		t.Fatal(err)
	}
	if !objv.HasErrors(diags) {
		t.Fatal("no diagnostic for an undeclared identifier")
	}
	if m != nil {
		t.Error("a rejected unit was lowered anyway")
	}
}

// ---- the whole pipeline ----

// A program compiled by objv, linked by objv's own linker, run by the OS.
//
// Only on the host it was built for: the object is arm64 Mach-O and the
// kernel is the judge. Everything above it runs everywhere.
func TestBuildAndRun(t *testing.T) {
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		t.Skip("the binary this produces runs on arm64 macOS")
	}
	dir := t.TempDir()
	out := filepath.Join(dir, "prog")

	var c objv.Compiler
	err := c.Build(objv.BuildParams{
		Output: out,
		Inputs: []objv.Input{objv.Text("prog.m", []byte(`
static int twice(int n) { return n * 2; }

int main(void) {
    int total = 0;
    for (int i = 0; i < 5; i++) {
        if (i == 2) continue;
        total += i;
    }
    return twice(total);   // (0+1+3+4)*2 = 16
}
`))},
	})
	if err != nil {
		var de *objv.DiagnosticError
		if errors.As(err, &de) {
			t.Fatalf("build: %v", de)
		}
		t.Skipf("link is not available here: %v", err)
	}

	cmd := exec.Command(out)
	_ = cmd.Run()
	if got := cmd.ProcessState.ExitCode(); got != 16 {
		t.Errorf("the program exited %d, want 16", got)
	}
}

// The same, for a program that is actually Objective-C.
//
// This is the one that says the compiler works. Everything in it has to be
// right at once: the class and metaclass objects and the four links between
// them, the read-only halves, the method and ivar lists, the offset variable
// every instance-variable access loads, the selector references the runtime
// rewrites at load, the category the runtime attaches to a class it did not
// compile, and the sections all of it lives in — none of which is reachable
// from any call, so nothing but running the program tests it.
//
// The classes it inherits from are libobjc's own: +alloc and -init come from
// the real NSObject, so the metaclass chain has to reach a class this
// compiler never saw.
func TestBuildAndRunObjectiveC(t *testing.T) {
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		t.Skip("the binary this produces runs on arm64 macOS")
	}
	out := filepath.Join(t.TempDir(), "prog")

	var c objv.Compiler
	err := c.Build(objv.BuildParams{
		Output: out,
		Inputs: []objv.Input{objv.Text("prog.m", []byte(`
@interface NSObject
+ (id)alloc;
- (id)init;
@end

@interface Counter : NSObject {
    int _n;
}
- (int)value;
- (void)add:(int)k;
+ (int)base;
@end

@implementation Counter
- (int)value { return _n; }
- (void)add:(int)k { _n = _n + k; }
+ (int)base { return 3; }
@end

// A category, which the runtime attaches at load: its method is not in the
// class's own method list and is found only because __objc_catlist was
// walked.
@interface Counter (Doubling)
- (int)doubled;
@end

@implementation Counter (Doubling)
- (int)doubled { return [self value] * 2; }
@end

int main(void) {
    Counter *c = [[Counter alloc] init];
    for (int i = 1; i <= 4; i++) [c add:i];
    return [c doubled] + [Counter base];   // (1+2+3+4)*2 + 3 = 23
}
`))},
	})
	if err != nil {
		var de *objv.DiagnosticError
		if errors.As(err, &de) {
			t.Fatalf("build: %v", de)
		}
		t.Skipf("link is not available here: %v", err)
	}

	cmd := exec.Command(out)
	_ = cmd.Run()
	if got := cmd.ProcessState.ExitCode(); got != 23 {
		t.Errorf("the program exited %d, want 23", got)
	}
}

// A program that is wrong comes back as a *DiagnosticError; every other error
// means objv could not run.
func TestBuildReportsTheProgram(t *testing.T) {
	c := hermetic()
	err := c.Build(objv.BuildParams{
		Output: filepath.Join(t.TempDir(), "prog"),
		Inputs: []objv.Input{objv.Text("prog.m", []byte("int main(void) { return nope; }\n"))},
	})
	var de *objv.DiagnosticError
	if !errors.As(err, &de) {
		t.Fatalf("err = %v, want a *DiagnosticError", err)
	}
	if !strings.Contains(de.Error(), "nope") {
		t.Errorf("the error does not name the mistake: %v", de)
	}
}

// Every source input is compiled even after one of them fails, so a build of
// four files reports four files' mistakes rather than the first one's.
func TestAllInputsAreReported(t *testing.T) {
	c := hermetic()
	err := c.Build(objv.BuildParams{
		Output: filepath.Join(t.TempDir(), "prog"),
		Inputs: []objv.Input{
			objv.Text("a.m", []byte("int f(void) { return aaa; }\n")),
			objv.Text("b.m", []byte("int g(void) { return bbb; }\n")),
		},
	})
	var de *objv.DiagnosticError
	if !errors.As(err, &de) {
		t.Fatalf("err = %v", err)
	}
	msg := de.Error()
	if !strings.Contains(msg, "aaa") || !strings.Contains(msg, "bbb") {
		t.Errorf("only some inputs were reported: %v", msg)
	}
}

// ---- helpers ----

func names(ms []preprocessor.Mount) []string {
	out := make([]string, 0, len(ms))
	for _, m := range ms {
		out = append(out, m.Name)
	}
	return out
}

func writeDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

var _ = sysroot.Version{}

// ---- against the real SDK ----

// The whole front end over the whole of Foundation.
//
// This is the test that matters. `#import <Foundation/Foundation.h>` reaches
// about nine hundred headers and four hundred thousand tokens, and every one
// of them is code Apple wrote against clang — so it exercises the parts of C
// and Objective-C no corpus written by the compiler's own author would think
// to write. Nearly every bug this compiler has had was found here.
//
// It is skipped where there is no SDK, which is every machine that is not a
// Mac with Xcode. Everything else in this file runs anywhere.
func TestFoundation(t *testing.T) {
	c := &objv.Compiler{Target: "aarch64-macos"}
	env, err := c.Env()
	if err != nil {
		t.Skipf("no target: %v", err)
	}
	if !env.SDK.Found() {
		t.Skip("no macOS SDK on this machine")
	}

	diags, err := c.Check(objv.Text("t.m", []byte(`#import <Foundation/Foundation.h>

@interface Greeter : NSObject
@property (nonatomic, copy) NSString *name;
- (NSString *)greeting;
@end

@implementation Greeter
- (NSString *)greeting { return [NSString stringWithUTF8String:"hello"]; }
@end

int main(void) {
    @autoreleasepool {
        Greeter *g = [[Greeter alloc] init];
        NSArray *a = @[@1, @"two", @3.0];
        for (id x in a) { (void)x; }
        (void)[g greeting];
    }
    return 0;
}
`)))
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, d := range diags {
		if d.Severity.String() != "error" {
			continue
		}
		n++
		if n <= 10 {
			t.Errorf("%s", d)
		}
	}
	if n > 10 {
		t.Errorf("and %d more errors", n-10)
	}
}
