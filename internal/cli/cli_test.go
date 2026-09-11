package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/vertex-language/objv"
)

// The command line's own rules, which the library's suite cannot see.
//
// The library is tested where it lives: the phases, the targets, the SDK, the
// link. What is left over is everything a wrapper decides — where an artifact
// lands, what -o means with two inputs, which exit code a verb returns, what
// a diagnostic looks like on a terminal — and Run takes its writers as
// arguments precisely so it can be tested here.

func run(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errb bytes.Buffer
	code = Run(args, &out, &errb)
	return code, out.String(), errb.String()
}

func write(t *testing.T, dir, name, src string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(src), 0o666); err != nil {
		t.Fatal(err)
	}
	return path
}

func hostOnly(t *testing.T) {
	t.Helper()
	if _, ok := objv.HostTarget(); !ok {
		t.Skip("this host is not a target objv models")
	}
}

// hermetic source: a root class and nothing imported, so every test below
// runs on a machine with no SDK.
const rootClass = `
__attribute__((objc_root_class))
@interface Counter
+ (int)base;
@end
@implementation Counter
+ (int)base { return 7; }
@end
int main(void) { return 0; }
`

func freestanding(args ...string) []string {
	return append([]string{args[0], "-freestanding"}, args[1:]...)
}

func TestUsage(t *testing.T) {
	code, _, stderr := run(t)
	if code != exitUsage {
		t.Errorf("no arguments: code = %d, want %d", code, exitUsage)
	}
	if !strings.Contains(stderr, "objv — the Vertex Objective-C compiler") {
		t.Error("no usage on stderr")
	}

	code, stdout, _ := run(t, "help")
	if code != exitOK || !strings.Contains(stdout, "objv build") {
		t.Errorf("help: code = %d, stdout = %q", code, stdout)
	}

	code, _, stderr = run(t, "frobnicate")
	if code != exitUsage || !strings.Contains(stderr, `unknown command "frobnicate"`) {
		t.Errorf("unknown verb: code = %d, stderr = %q", code, stderr)
	}

	code, stdout, _ = run(t, "version")
	if code != exitOK || !strings.Contains(stdout, objv.Version) {
		t.Errorf("version: code = %d, stdout = %q", code, stdout)
	}
}

// An unknown target names the flag that fixes it and the targets that exist.
// That is the one thing a library cannot say and the one thing a person at a
// terminal wants to read.
func TestUnknownTarget(t *testing.T) {
	code, _, stderr := run(t, "check", "-target", "pdp11-unix", "-")
	if code != exitUsage {
		t.Errorf("code = %d, want %d", code, exitUsage)
	}
	if !strings.Contains(stderr, "unknown target") || !strings.Contains(stderr, "aarch64-macos") {
		t.Errorf("stderr = %q", stderr)
	}
}

// check reports diagnostics on stderr, prints the source line and a caret
// under it, and exits 1.
func TestCheckDiagnostics(t *testing.T) {
	hostOnly(t)
	dir := t.TempDir()
	src := write(t, dir, "bad.m", "int f(void) { return nope; }\n")

	code, stdout, stderr := run(t, freestanding("check", src)...)
	if code != exitDiags {
		t.Errorf("code = %d, want %d", code, exitDiags)
	}
	if stdout != "" {
		t.Errorf("check wrote to stdout: %q", stdout)
	}
	if !strings.Contains(stderr, "'nope' is undeclared") {
		t.Errorf("stderr = %q", stderr)
	}
	// The snippet and its caret.
	if !strings.Contains(stderr, "int f(void) { return nope; }") {
		t.Error("no source line under the diagnostic")
	}
	if !strings.Contains(stderr, "^^^^") {
		t.Error("no caret under the diagnostic")
	}
}

// Every input is checked even after one of them fails, so a check of two
// files reports both files' mistakes.
func TestCheckReportsEveryInput(t *testing.T) {
	hostOnly(t)
	dir := t.TempDir()
	a := write(t, dir, "a.m", "int f(void) { return aaa; }\n")
	b := write(t, dir, "b.m", "int g(void) { return bbb; }\n")

	code, _, stderr := run(t, freestanding("check", a, b)...)
	if code != exitDiags {
		t.Errorf("code = %d", code)
	}
	if !strings.Contains(stderr, "aaa") || !strings.Contains(stderr, "bbb") {
		t.Errorf("only some inputs were reported: %q", stderr)
	}
}

// A clean file is exit 0 and silence, so `objv check f.m && echo ok` means
// what it looks like.
func TestCheckClean(t *testing.T) {
	hostOnly(t)
	dir := t.TempDir()
	src := write(t, dir, "ok.m", rootClass)

	code, stdout, stderr := run(t, freestanding("check", src)...)
	if code != exitOK {
		t.Errorf("code = %d, stderr = %q", code, stderr)
	}
	if stdout != "" || stderr != "" {
		t.Errorf("a clean check said something: %q %q", stdout, stderr)
	}
}

// --emit mi with one input goes to standard output, which is what a pipe
// expects.
func TestEmitMIToStdout(t *testing.T) {
	hostOnly(t)
	dir := t.TempDir()
	src := write(t, dir, "m.m", "#define TWICE(x) ((x) * 2)\nint f(int n) { return TWICE(n); }\n")

	code, stdout, stderr := run(t, freestanding("build", "--emit", "mi", src)...)
	if code != exitOK {
		t.Fatalf("code = %d, stderr = %q", code, stderr)
	}
	if strings.Contains(stdout, "TWICE") {
		t.Errorf("the macro did not expand:\n%s", stdout)
	}
	if !strings.Contains(stdout, "return ( ( n ) * 2 )") &&
		!strings.Contains(stdout, "((n) * 2)") {
		t.Errorf("unexpected output:\n%s", stdout)
	}
}

// --emit vir names its artifact for its input when there is more than one,
// and puts it in the working directory rather than beside the source — which
// is where cc -c puts one.
func TestEmitVIRNamesArtifacts(t *testing.T) {
	hostOnly(t)
	dir := t.TempDir()
	a := write(t, dir, "one.m", "int f(void) { return 1; }\n")
	b := write(t, dir, "two.m", "int g(void) { return 2; }\n")

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	code, _, stderr := run(t, freestanding("build", "--emit", "vir", a, b)...)
	if code != exitOK {
		t.Fatalf("code = %d, stderr = %q", code, stderr)
	}
	for _, name := range []string{"one.vir", "two.vir"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if !strings.Contains(string(data), "module ") {
			t.Errorf("%s does not look like VIR:\n%s", name, data)
		}
	}
}

// -o with more than one input, for a per-input artifact, is a mistake with
// one answer: say so rather than write one file twice.
func TestEmitRejectsOneOutputForManyInputs(t *testing.T) {
	hostOnly(t)
	dir := t.TempDir()
	a := write(t, dir, "a.m", "int f(void) { return 1; }\n")
	b := write(t, dir, "b.m", "int g(void) { return 2; }\n")

	code, _, stderr := run(t, freestanding("build", "--emit", "obj", "-o",
		filepath.Join(dir, "x.o"), a, b)...)
	if code != exitUsage {
		t.Errorf("code = %d, want %d", code, exitUsage)
	}
	if !strings.Contains(stderr, "one artifact per input") {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestUnknownEmit(t *testing.T) {
	code, _, stderr := run(t, "build", "--emit", "bitcode", "-")
	if code != exitUsage || !strings.Contains(stderr, `unknown --emit "bitcode"`) {
		t.Errorf("code = %d, stderr = %q", code, stderr)
	}
}

// An object file needs a path: -o - is for the text artifacts.
func TestObjectNeedsAPath(t *testing.T) {
	hostOnly(t)
	dir := t.TempDir()
	src := write(t, dir, "a.m", "int f(void) { return 1; }\n")

	code, _, stderr := run(t, freestanding("build", "--emit", "obj", "-o", "-", src)...)
	if code != exitUsage || !strings.Contains(stderr, "needs a path") {
		t.Errorf("code = %d, stderr = %q", code, stderr)
	}
}

// env prints the resolved lists, which is the invariant the READMEs promise:
// header search is data, inspectable before the build runs.
func TestEnv(t *testing.T) {
	hostOnly(t)
	code, stdout, stderr := run(t, "env", "-freestanding")
	if code != exitOK {
		t.Fatalf("code = %d, stderr = %q", code, stderr)
	}
	for _, want := range []string{"target:", "hosted: false", "search:", "<builtin>"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("env output has no %q:\n%s", want, stdout)
		}
	}
	if strings.Contains(stdout, "predefines:") {
		t.Error("env printed predefines without -defines")
	}

	_, stdout, _ = run(t, "env", "-freestanding", "-defines")
	if !strings.Contains(stdout, "-D __CHAR_BIT__=8") {
		t.Errorf("env -defines has no model macros:\n%s", stdout)
	}
}

func TestEnvTakesNoFiles(t *testing.T) {
	code, _, stderr := run(t, "env", "x.m")
	if code != exitUsage || !strings.Contains(stderr, "takes no files") {
		t.Errorf("code = %d, stderr = %q", code, stderr)
	}
}

// tokens dumps the stream the parser will read, with positions in the file
// the user wrote.
func TestTokens(t *testing.T) {
	hostOnly(t)
	dir := t.TempDir()
	src := write(t, dir, "t.m", "@interface K @end\n")

	code, stdout, stderr := run(t, freestanding("tokens", src)...)
	if code != exitOK {
		t.Fatalf("code = %d, stderr = %q", code, stderr)
	}
	// A kind prints as its spelling, which is what the token package's
	// String does: @interface is one token and there is nothing else to
	// call it.
	for _, want := range []string{"@interface", "IDENT", "K", "@end", "EOF"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("tokens output has no %q:\n%s", want, stdout)
		}
	}
}

func TestASTDumpsATree(t *testing.T) {
	hostOnly(t)
	dir := t.TempDir()
	src := write(t, dir, "a.m", rootClass)

	code, stdout, stderr := run(t, freestanding("ast", src)...)
	if code != exitOK {
		t.Fatalf("code = %d, stderr = %q", code, stderr)
	}
	for _, want := range []string{"ClassInterfaceDecl", "ClassImplDecl", "MethodDecl"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("ast output has no %q:\n%s", want, stdout)
		}
	}
}

func TestASTOneFileAtATime(t *testing.T) {
	code, _, stderr := run(t, "ast", "a.m", "b.m")
	if code != exitUsage || !strings.Contains(stderr, "one file at a time") {
		t.Errorf("code = %d, stderr = %q", code, stderr)
	}
}

// run builds into a temporary directory and forwards the program's exit
// code: a program that exits 3 did not fail to build.
func TestRunForwardsExitCode(t *testing.T) {
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		t.Skip("the binary this produces runs on arm64 macOS")
	}
	dir := t.TempDir()
	src := write(t, dir, "p.m", `
__attribute__((objc_root_class))
@interface K
+ (int)n;
@end
@implementation K
+ (int)n { return 3; }
@end
int main(void) { return [K n]; }
`)
	code, _, stderr := run(t, "run", src)
	if code != 3 {
		t.Errorf("code = %d, want the program's 3; stderr = %q", code, stderr)
	}
}

// Running a program for another machine is not this machine's business, and
// the message says which half is the problem.
func TestRunRefusesACrossTarget(t *testing.T) {
	hostOnly(t)
	other := "x86_64-linux"
	if objv.HostName() == other {
		other = "aarch64-macos"
	}
	code, _, stderr := run(t, "run", "-target", other, "p.m")
	if code != exitUsage || !strings.Contains(stderr, "is not this machine") {
		t.Errorf("code = %d, stderr = %q", code, stderr)
	}
}

func TestRunNeedsAFile(t *testing.T) {
	hostOnly(t)
	code, _, stderr := run(t, "run")
	if code != exitUsage || !strings.Contains(stderr, "needs a file") {
		t.Errorf("code = %d, stderr = %q", code, stderr)
	}
}

// build links an executable and runs it, which is the whole pipeline through
// the command a person actually types.
func TestBuildLinksAndRuns(t *testing.T) {
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		t.Skip("the binary this produces runs on arm64 macOS")
	}
	dir := t.TempDir()
	src := write(t, dir, "p.m", `
__attribute__((objc_root_class))
@interface K
+ (int)n;
@end
@implementation K
+ (int)n { return 9; }
@end
int main(void) { return [K n]; }
`)
	exe := filepath.Join(dir, "prog")
	code, _, stderr := run(t, "build", "-o", exe, src)
	if code != exitOK {
		t.Fatalf("build: code = %d, stderr = %q", code, stderr)
	}
	if _, err := os.Stat(exe); err != nil {
		t.Fatalf("no executable: %v", err)
	}
}

func TestArtifactName(t *testing.T) {
	for _, c := range []struct{ in, ext, want string }{
		{"src/Counter.m", ".o", "Counter.o"},
		{"Counter.m", ".vir", "Counter.vir"},
		{"-", ".o", "a.o"},
		{"", ".mi", "a.mi"},
		{"a/b/c.mi", "", "c"},
	} {
		if got := artifactName(c.in, c.ext); got != c.want {
			t.Errorf("artifactName(%q, %q) = %q, want %q", c.in, c.ext, got, c.want)
		}
	}
}

func TestSplitDashDash(t *testing.T) {
	mine, theirs := splitDashDash([]string{"-o", "x", "--", "-v", "arg"})
	if strings.Join(mine, " ") != "-o x" || strings.Join(theirs, " ") != "-v arg" {
		t.Errorf("mine = %v, theirs = %v", mine, theirs)
	}
	mine, theirs = splitDashDash([]string{"a.m"})
	if len(mine) != 1 || theirs != nil {
		t.Errorf("mine = %v, theirs = %v", mine, theirs)
	}
}
