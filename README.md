<p align="center">
  <img src="https://img.shields.io/badge/spec-draft-2563EB?style=flat-square&amp;labelColor=0D1030" alt="Spec draft" />
  <img src="https://img.shields.io/badge/compiler-0.0.0-4F46E5?style=flat-square&amp;labelColor=0D1030" alt="Compiler 0.0.0" />
  <img src="https://img.shields.io/badge/go-1.23%2B-7C3AED?style=flat-square&amp;labelColor=0D1030" alt="Go 1.23+" />
  <img src="https://img.shields.io/badge/license-MIT-9333EA?style=flat-square&amp;labelColor=0D1030" alt="MIT License" />
</p>

# objv

**objv** is the Vertex Objective-C compiler: a from-scratch front end and native backend written in Go, with no dependency on a host `cc`, `as`, or `ld`. It reads Objective-C — or preprocessed `.mi` — and writes a linked executable, an object file, or VIR, the shared typed IR. The preprocessor, code generator, object encoders, and linkers are all built in, so cross-compiling and cross-linking require no external toolchain installed.

It is both a standalone CLI and a Go library sharing the same pipeline: `objv build` and `objv.Build("app", "app.m")` execute identical logic.

The language is Objective-C on a C11 substrate: classes, categories, protocols, properties, blocks, lightweight generics, `@try`/`@throw`, ARC bridge casts, and the extensions modern Cocoa and system headers require. [`docs/objc_grammar.md`](docs/objc_grammar.md) serves as the formal language specification.

---

- [Install](#install)
- [Quick start](#quick-start)
- [Language](#language)
- [The object model](#the-object-model)
- [Targets](#targets)
- [CLI](#cli)
- [Go API](#go-api)
- [Architecture](#architecture)

---

## Install

```console
$ GOPROXY=direct go install github.com/vertex-language/objv/cmd/objv@latest
```

Requires Go 1.23 or newer. No external toolchain or SDK is required to build the compiler itself.

---

## Quick start

```objc
/* hello.m */
#import <Foundation/Foundation.h>

@interface Greeter : NSObject
- (void)greet:(NSString *)name;
@end

@implementation Greeter
- (void)greet:(NSString *)name {
    NSLog(@"Hello, %@!", name);
}
@end

int main(int argc, const char *argv[]) {
    @autoreleasepool {
        Greeter *greeter = [[Greeter alloc] init];
        [greeter greet:@"World"];
    }
    return 0;
}
```

```console
$ objv build -o hello hello.m
$ ./hello
Hello, World!

$ objv run hello.m
Hello, World!

$ objv check hello.m && echo ok
ok
```

Running programmatically via Go:

```go
import "github.com/vertex-language/objv"

err := objv.Build("hello", "hello.m")   // compile and link
out, err := objv.Run("hello.m")         // build to temp path and run
```

Stopping after lowering to inspect VIR (Vertex Intermediate Representation):

```console
$ objv build --emit vir -o - hello.m
```

```text
export func @Greeter_greet(%self ptr, %_cmd ptr, %name ptr) void nounwind {
@entry:
  %fmt = ptr.const @_OBJC_STRING_0
  call @NSLog(%fmt, %name)
  return
}
```

---

## Language

`objv` compiles Objective-C against a C11 core:

- **Object Model:** Classes, metaclasses, categories, class extensions, instance variables, and dynamic properties (`@synthesize`, `@dynamic`).
- **Protocols & Typing:** Protocol conformances, optional and required methods, qualified types (`id<NSCopying>`), and lightweight generics (`NSArray<NSString *> *`).
- **Literals & Subscripting:** Object, array, dictionary, and boxed number literals (`@"string"`, `@[...]`, `@{...}`, `@42`), along with keyed and indexed subscripting (`dict[key]`, `arr[idx]`).
- **Memory Management:** Automatic Reference Counting (ARC) with retain/release lifecycle synthesis, `@autoreleasepool` blocks, and bridge casts (`__bridge`, `__bridge_transfer`, `__bridge_retained`).
- **Blocks:** Block literal syntax, capture analysis, closure layout emission, and runtime descriptor setup. A literal that captures nothing is a global block; one that captures is built in the frame, with the copy and dispose helpers a captured object needs.
- **Control & Exceptions:** Fast enumeration (`for (id item in collection)`), and Objective-C exceptions (`@try`, `@catch`, `@finally`, `@throw`).
- **Extensions:** GCC and Clang attributes required by standard system headers (`__attribute__((objc_root_class))`, `unavailable`, `deprecated`, nullability annotations).

Unknown extensions outside standard system usage are rejected with informative diagnostics rather than silently accepted or reinterpreted.

---

## The object model

Objective-C is fundamentally dynamic: message dispatches and metadata structures define the language semantics. `objv` treats emission and metadata layout as primary design concerns:

- **Message sends:** Lowering `[receiver selector:arg]` into runtime dispatches (`objc_msgSend`). Specialised return variants (`objc_msgSend_stret`, `objc_msgSend_fpret`) are selected based on target ABI requirements.
- **Class and metaclass metadata:** Emission of `_OBJC_CLASS_$_*` and `_OBJC_METACLASS_$_*` structures, method lists, ivar offsets, protocol reference tables, and property attributes.
- **Runtime sections:** Dedicated Mach-O and ELF section placement for runtime consumption (`__objc_classlist`, `__objc_catlist`, `__objc_protolist`, `__objc_classrefs`, `__objc_selrefs`).
- **ARC lowering:** Static ownership analysis placed directly at the VIR lowering stage, balancing retain/release calls and autorelease pool boundaries prior to instruction selection.

---

## Targets

Native cross-compilation is supported without requiring a target-specific host toolchain. Object writers and linkers are internal.

| Target | Container | Notes |
|---|---|---|
| `aarch64-macos` | Mach-O | Apple Silicon macOS native |
| `x86_64-macos` | Mach-O | Intel macOS native |
| `aarch64-linux` | ELF | Requires libobjc2 / GNUstep runtime |
| `x86_64-linux` | ELF | Requires libobjc2 / GNUstep runtime |
| `x86_64-windows` | PE / COFF | Win32 / GNUstep runtime layout |

Select an architecture and OS target using the `-target` flag:

```console
$ objv build -target aarch64-macos -o app main.m
```

Frameworks and search paths are controlled via standard flags:
- `-framework <Name>`: Link against a platform framework
- `-F <dir>`: Add directory to framework search path
- `-isysroot <path>`: Direct the preprocessor and linker to platform headers and libraries

---

## CLI

Commands use verb-first dispatch:

| Command | Description |
|---|---|
| `objv build` | Compile and link input files into an executable |
| `objv run` | Compile, link to a temporary binary, execute, and forward the exit code |
| `objv check` | Parse, validate types, and run semantic checks without writing an artifact |
| `objv ast` | Parse input files and print the generated syntax tree |
| `objv tokens` | Tokenize the input stream and dump tokens with source positions |
| `objv env` | Print active target settings, the SDK, search paths, and predefined macros |
| `objv version` | Print the compiler's version |

### Target artifact control (`--emit`)

The `--emit` flag specifies the terminal stage of compilation:

| Option | Emitted Output |
|---|---|
| `--emit exe` | Linked executable binary (default) |
| `--emit obj` | Unlinked object file (`.o`) |
| `--emit vir` | Lowered Vertex Intermediate Representation |
| `--emit mi` | Preprocessed Objective-C source |

Standard compiler options apply: `-o`, `-I`, `-F`, `-D`, `-U`, `-L`, `-l`,
`-framework`, `-include`, `-isysroot` and `-static`, plus
`-fobjc-arc` (automatic reference counting), `-mmacosx-version-min`
(the oldest macOS the build runs on) and `-freestanding` (objv's builtin
headers alone).

A file of `-`, or no file, reads standard input — except for `objv run`,
whose program inherits standard input and so cannot take its source from
there. `-pp` and `-no-pp` override what the extension says about
preprocessing.

---

## Go API

The root package exports the compiler interface:

```go
import (
	"github.com/vertex-language/objv"
	"github.com/vertex-language/objv/preprocessor"
)

c := &objv.Compiler{
	Target:        "aarch64-macos",
	FrameworkDirs: []string{"/Library/Frameworks"},
	Defines:       []preprocessor.Predefine{objv.Define("DEBUG", "1")},
}

err := c.Build(objv.BuildParams{
	Output:     "output",
	Inputs:     []objv.Input{objv.File("main.m"), objv.File("helper.m")},
	Frameworks: []string{"Foundation"},
})
```

Front-end packages can be imported separately for static analysis or code transformation:

```go
import (
	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/parser"
	"github.com/vertex-language/objv/token"
)

unit := token.NewFile("main.m", src)
file, diags := parser.ParseFile(unit, parser.DefaultMode)

ast.Inspect(file, func(n ast.Node) bool {
	if cls, ok := n.(*ast.ClassInterfaceDecl); ok {
		// inspect class declaration
	}
	return true
})
```

---

## Architecture

The pipeline executes in sequential stages:

```
scanner -> preprocessor -> parser -> analyzer -> lower -> objv
 @-tokens      #import        AST       types, ARC,   typed    isel, encode,
               macros                  classes,      AST->VIR link
                                       protocols
```

| Package | Purpose |
|---|---|
| `token/` | Lexical tokens, source coordinates, and file tracking |
| `scanner/` | Tokenizer handling `@` keywords, string literals, and selector tokens |
| `preprocessor/` | Directive resolution, macro evaluation, `#import` tracking, framework inclusion |
| `parser/` · `ast/` | Syntax parsing and AST generation for Objective-C and C constructs |
| `analyzer/` · `types/` | Symbol tables, class hierarchies, method signatures, protocols, ARC semantics |
| `lower/` | AST to VIR translation; lowering message sends, block captures, and `@autoreleasepool` |
| `runtime/` | Objective-C runtime ABI structures and runtime symbol emission |
| `sysroot/` | Framework, SDK, and platform include/library discovery; deployment target and platform predefines |
| `internal/cli/` · `cmd/objv/` | CLI command parsing, flag binding, and terminal dispatch |
| `.` | Core compiler coordinator: target resolution, predefines, pipeline management, backend orchestration |

---

MIT licensed. See [LICENSE](LICENSE).