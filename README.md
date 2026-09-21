# objv

<p align="center">
  <img src="https://img.shields.io/badge/language-Objective--C-2563EB?style=flat-square&labelColor=0D1030" alt="Objective-C">
  <img src="https://img.shields.io/badge/compiler-0.0.0--dev-4F46E5?style=flat-square&labelColor=0D1030" alt="0.0.0-dev">
  <img src="https://img.shields.io/badge/go-1.23%2B-7C3AED?style=flat-square&labelColor=0D1030" alt="Go 1.23+">
  <img src="https://img.shields.io/badge/license-MIT-9333EA?style=flat-square&labelColor=0D1030" alt="MIT License">
</p>

**objv** is the Vertex Objective-C compiler. It provides a from-scratch Objective-C (C11 substrate) front end and native code generator written in Go, lowering directly to VIR (Vertex Intermediate Representation) and linking native executables and object files (`.o`).

The CLI binary is `objv`; the Go module and repository is `github.com/vertex-language/objv`.

```console
$ objv build -o hello hello.m
$ objv run hello.m
$ objv check src/
$ objv build --emit vir hello.m
$ objv ast hello.m
$ objv tokens hello.m
$ objv env
```

---

## Highlights

- **From-Scratch Objective-C Front End**: Independent lexer, ISO phase 4 preprocessor with native `#import` and `@import`, recursive-descent parser, semantic analyzer, and type system.
- **Modern Language Capabilities**: Full support for Automatic Reference Counting (ARC), block closures, lightweight generics, fast enumeration (`for..in`), `@try`/`@catch`/`@finally` exceptions, boxed literals, and indexed/keyed subscripting.
- **Complete Runtime & ABI Parity**: Emits Apple 64-bit Objective-C runtime metadata (class and metaclass descriptors, method lists, ivar offsets, protocol tables, selector references) and supports libobjc2 / GNUstep runtimes.
- **Direct Lowering to VIR**: Lowers dynamic message sends (`objc_msgSend` variants), ARC ownership lifecycles, and closure frames directly into typed SSA VIR modules without intermediate external C or LLVM layers.
- **Self-Contained Native Pipeline**: Integrated code generators, object encoders (Mach-O, ELF, PE), and platform linkers built in Go—no dependency on host `clang`, `gcc`, `as`, or `ld` for compilation or cross-linking.
- **Rich Introspection CLI**: Built-in commands to dump token streams, syntax tree representations (AST), lowered VIR modules, preprocessed output, and resolved SDK search paths.
- **Modular Go Library**: Every compilation stage is exposed as a composable, reusable Go API.

---

## Quick Start

### Installation

Compile and install `objv` using the Go toolchain:

```console
$ go install github.com/vertex-language/objv/cmd/objv@latest
```

Or build directly from source:

```console
$ git clone https://github.com/vertex-language/objv.git
$ cd objv
$ go build -o objv ./cmd/objv
```

### Basic Usage

Write an Objective-C source file:

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

Compile, check, and run:

```console
# Compile and link to a native executable
$ objv build -o hello hello.m
$ ./hello
Hello, World!

# Build to a temporary directory and execute directly
$ objv run hello.m
Hello, World!

# Type check without generating an artifact
$ objv check hello.m

# Inspect the lowered Vertex Intermediate Representation (VIR)
$ objv build --emit vir hello.m
```

The lowered VIR module represents dynamic dispatch, strings, and ownership cleanups in clean SSA form:

```text
export func @Greeter_greet(%self ptr, %_cmd ptr, %name ptr) void nounwind {
@entry:
  %fmt = ptr.const @_OBJC_STRING_0
  call @NSLog(%fmt, %name)
  return
}
```

---

## CLI Reference

`objv` uses a verb-first command structure:

```
objv <command> [flags] [files...]
```

A file of `-` (or omitting files) reads standard input, except for `objv run` (where the executed program inherits standard input).

### Commands

| Command | Description |
| --- | --- |
| `objv build` | Compile and link input files into an executable or intermediate artifact. |
| `objv run` | Compile inputs to a temporary binary and execute it, forwarding arguments after `--`. |
| `objv check` | Run preprocessing, parsing, and semantic analysis; print diagnostics with source snippets. |
| `objv ast` | Parse inputs and dump the abstract syntax tree. |
| `objv tokens` | Tokenize the input and dump the token stream with source line/column positions. |
| `objv env` | Print active target, SDK paths, search paths, and predefined macros. |
| `objv version` | Print the compiler's version. |

### Target Artifact Control (`--emit`)

Used with `objv build` to stop after a specific phase:

| Option | Emitted Output |
| --- | --- |
| `--emit exe` | Compile and link a native executable binary (default). |
| `--emit obj` | Compile to a relocatable native object file (`.o`), one per input. |
| `--emit vir` | Stop after lowering and output lowered Vertex Intermediate Representation. |
| `--emit mi` | Stop after preprocessing and print preprocessed source. |

### Common Flags

- `-target <triple>`: Target platform to compile for (e.g. `aarch64-macos`, `x86_64-macos`, `aarch64-linux`, `x86_64-linux`, `x86_64-windows`). Defaults to the host system.
- `-o <file>`: Output path (`-` writes to standard output for `vir` and `mi`).
- `-I <dir>`: Add an include search directory (repeatable, searched in order).
- `-F <dir>`: Add a framework search directory (repeatable, searched in order).
- `-D <name>[=<val>]`: Define a preprocessor macro.
- `-U <name>`: Undefine a preprocessor macro.
- `-include <file>`: Process a header before the main input.
- `-isysroot <path>`: Specify the platform SDK directory.
- `-mmacosx-version-min <v>`: Specify deployment target (oldest macOS release supported).
- `-fobjc-arc`: Compile with Automatic Reference Counting.
- `-freestanding`: Use builtin headers only; disable SDK and framework discovery.
- `-pp` / `-no-pp`: Force preprocessing on or off (overrides file extensions like `.m` vs `.mi`).

### Linker Flags (for `build` and `run`)

- `-L <dir>`: Add a library search directory (repeatable, searched in order).
- `-l <name>`: Link against a library (e.g. `-lobjc`).
- `-framework <name>`: Link against a platform framework (e.g. `-framework Foundation`).
- `-entry <sym>`: Override entry symbol (defaults to platform entry, e.g. `_main` or `main`).
- `-static`: Link a static executable image.

### Inspection Flags

- `-skip-bodies` (for `ast`): Skip parsing function and method bodies for fast structural scanning.
- `-comments` (for `ast`): Retain comments on AST nodes.
- `-defines` (for `env`): Print active predefined macros.

---

## Compiler Architecture

Translation follows a multi-phase compiler pipeline implemented across isolated, modular Go packages:

```
                     Source Text (.m / .mi)
                               │
                       [ token / scanner ]
                        Phases 1, 2, 3
                    (Tokens, Sites, @-Tokens)
                               │
                       [ preprocessor ]
                           Phase 4
                 (#import, Directives, Macros)
                               │
                           [ parser ]
                             Phase 7
                        (Syntax Tree AST)
                               │
                      [ analyzer / types ]
                           Phases 7, 8
                  (Classes, Protocols, ARC, Type Check)
                               │
                      [ lower / runtime ]
                            Phase 8
                (VIR Lowering, ABI Metadata, Closures)
                               │
                           VIR Module
                               │
                  [ Native Backend & Linker ]
                            Phase 9
                      (Mach-O / ELF / PE)
                               │
                       Native Executable
```

### Compilation Phases

1. **Tokens & Scanning (`token`, `scanner`)**:
   Phases 1–3: Trigraph replacement, line splicing, source coordinate mapping (`token.Pos`, `token.File`), tokenization, Objective-C `@` punctuators, string literals (`@"..."`), and number literal classification.
2. **Preprocessing (`preprocessor`, `sysroot`)**:
   Phase 4: Full macro expansion, stringification (`#`), token pasting (`##`), conditional compilation (`#if`, `#ifdef`), header caching, `#import` idempotency, `@import` module directives, and platform framework discovery.
3. **Parsing (`parser`, `ast`)**:
   Phase 7: Recursive-descent parser producing an explicit AST for C11 declarations, expressions, statements, and Objective-C extensions (`@interface`, `@implementation`, `@protocol`, `@property`, `@autoreleasepool`, `@try`). Resilient recovery ensures partial ASTs are generated even for invalid source.
4. **Semantic Analysis (`analyzer`, `types`)**:
   Type checking, class and category symbol tables, protocol conformance verification, method dispatch resolution, lightweight generics validation, property synthesis, and ARC ownership rules.
5. **Lowering & Runtime ABI (`lower`, `runtime`)**:
   Lowers the elaborated AST to Vertex Intermediate Representation (VIR). Emits dynamic dispatch calls (`objc_msgSend`), ARC retain/release/autorelease insertions, `.cxx_destruct` methods, closure capture frames, and runtime metadata structures (`_OBJC_CLASS_$_*`, method lists, ivar offset tables).
6. **Native Code Generation & Linking (`build`, `link`)**:
   Direct translation from VIR into machine instructions and native container formats (`macho`, `elf`, `pe`) using Vertex architecture encoders (`arm64`, `amd64`). Built-in linkers produce final executables without invoking host linkers.

---

## The Objective-C Runtime Model

`objv` treats runtime emission and ABI compliance as core design requirements:

### Dynamic Message Dispatch
Message expressions `[receiver selector:arg]` lower to runtime dispatches:
- Standard dispatch: `objc_msgSend(receiver, op, ...)`
- Super dispatch: `objc_msgSendSuper(super, op, ...)`
- Structure return: `objc_msgSend_stret` (when target ABI returns aggregates via memory)
- Floating-point return: `objc_msgSend_fpret` (on architectures requiring special FP return conventions)

### Metadata Sections
Generates native object sections matching platform runtime conventions:
- `__objc_classlist`: Pointers to root and subclass structures
- `__objc_catlist`: Categories extending existing classes
- `__objc_protolist`: Protocols declared in the translation unit
- `__objc_classrefs` & `__objc_superrefs`: External and local class references
- `__objc_selrefs`: Unique selector references

### Automatic Reference Counting (ARC)
ARC lowering performs static ownership tracking during VIR emission:
- Methods named `alloc`, `init`, `copy`, `mutableCopy`, and `new` return retain count `+1`.
- Retain, release, and autorelease operations are balanced and inserted directly in SSA IR.
- Classes with object ivars automatically receive `.cxx_destruct` methods.
- Bridge casts (`__bridge`, `__bridge_transfer`, `__bridge_retained`) control ownership transfer between CoreFoundation and Objective-C pointers.

### Blocks ABI
- Capture analysis determines whether a block can be emitted as a global singleton or requires a stack/heap frame.
- Stack frames synthesize runtime block descriptors, capture offsets, and dedicated copy/dispose helper functions for captured object references.

---

## Targets and Platforms

`objv` models target architectures, object container formats, and runtime ABIs:

| Target | Architecture | OS | Container | Runtime ABI | Default Search / SDK |
| --- | --- | --- | --- | --- | --- |
| `aarch64-macos` | AArch64 | macOS | Mach-O | Apple 64-bit v2 | macOS SDK (`Xcode.app` / `CommandLineTools`) |
| `x86_64-macos` | x86_64 | macOS | Mach-O | Apple 64-bit v2 | macOS SDK (`Xcode.app` / `CommandLineTools`) |
| `aarch64-linux` | AArch64 | Linux | ELF | GNUstep / libobjc2 | System `/usr/include`, `/usr/lib` |
| `x86_64-linux` | x86_64 | Linux | ELF | GNUstep / libobjc2 | System `/usr/include`, `/usr/lib` |
| `x86_64-windows` | x86_64 | Windows | PE/COFF | Win32 / GNUstep | Windows SDK / GNUstep headers |

Target selection is fully cross-capable:
```console
# Cross-compile for Linux from macOS without host cross-tools
$ objv build -target x86_64-linux -o app.o --emit obj app.m
```

---

## Go API

`objv` provides a high-level driver and exposes every compilation stage as a reusable Go library:

```go
package main

import (
	"fmt"
	"log"

	"github.com/vertex-language/objv"
	"github.com/vertex-language/objv/preprocessor"
)

func main() {
	// Simple one-line build
	if err := objv.Build("hello", "hello.m"); err != nil {
		log.Fatalf("build failed: %v", err)
	}

	// Advanced configuration via Compiler driver
	c := &objv.Compiler{
		Target:        "aarch64-macos",
		ARC:           true,
		FrameworkDirs: []string{"/Library/Frameworks"},
		Defines: []preprocessor.Predefine{
			{Kind: preprocessor.PredefineDefine, Text: "DEBUG=1"},
		},
	}

	// Check translation unit for diagnostics
	diags, err := c.Check(objv.File("hello.m"))
	if err != nil {
		log.Fatal(err)
	}
	for _, d := range diags {
		fmt.Println(d.String())
	}

	// Compile and link executable
	res, err := c.Compile(objv.BuildParams{
		Output:     "hello",
		Inputs:     []objv.Input{objv.File("hello.m")},
		Frameworks: []string{"Foundation"},
	})
	if err != nil {
		log.Fatalf("compilation error: %v", err)
	}
	fmt.Printf("Compiled %d bytes successfully.\n", len(res.Artifact))
}
```

### In-Memory Compilation

Source inputs can be passed directly from memory without touching the filesystem:

```go
src := []byte(`
#import <objc/runtime.h>
int main(void) {
    return 0;
}
`)

c := &objv.Compiler{Target: objv.HostName()}
unit, diags, err := c.Check(objv.Text("scratch.m", src))
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Diagnostics: %d\n", len(diags))
```

### AST Inspection

```go
import (
	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/parser"
	"github.com/vertex-language/objv/token"
)

unit := token.NewFile("source.m", src)
file, diags := parser.ParseFile(unit, parser.DefaultMode)

ast.Inspect(file.Unit, func(n ast.Node) bool {
	if decl, ok := n.(*ast.ClassInterfaceDecl); ok {
		fmt.Printf("Declared class: %s\n", decl.Name.Name)
	}
	return true
})
```

---

## Repository Structure

```
objv/
├── cmd/objv/           Binary entry point for the objv CLI tool
├── internal/cli/       CLI command dispatch, flag definitions, and formatting
│
├── token/              Phases 1–2: tokens, source coordinates (Pos), file buffers
├── scanner/            Phase 3: tokenization, @-tokens, literals, comments
├── preprocessor/       Phase 4: directives, macros, #import, header search
├── parser/             Phase 7: recursive-descent C11 and Objective-C parser
├── ast/                Syntax tree node definitions and tree inspection utilities
│
├── types/              Type representations, size/alignment models, layout engines
├── analyzer/           Semantic analysis, scopes, protocols, method lookup, ARC
├── lower/              Lowering from AST to Vertex Intermediate Representation (VIR)
├── runtime/            Runtime ABI symbols, metadata emission, section generation
├── sysroot/            Embedded freestanding headers, SDK discovery, platform predefines
│
├── compiler.go         Central Compiler driver orchestrating pipeline stages
├── build.go            Native object generation and code emission
├── link.go             Executable linker orchestration
├── target.go           Target triples, architectures, and ABI mappings
├── objv.go             Top-level entry points and pipeline coordinator
│
├── tests/              001–200: one construct per program, built by objv and clang and compared
└── docs/               Formal language grammar specification
```

---

## Testing & Conformance

`objv` uses four test corpora organized by compiler layer:

```console
$ go test ./...                                    # Run the test suite
$ go test ./parser       -run TestSyntaxCorpus -v  # Grammar syntax coverage
$ go test ./analyzer     -run TestCheckCorpus -v   # Semantic type checking
$ go test ./lower        -run TestEvalCorpus -v    # VIR lowering assertions
$ go test -run TestPrograms .                      # End-to-end parity vs clang
```

- **`syntax/`**: Validates parser conformance against the Objective-C grammar chapters. Each file is verified against `clang -fsyntax-only` as an oracle.
- **`check/`**: Verifies type checking and diagnostic precision. Files specify exact expected errors and positions via `// expect: <message>`.
- **`eval/`**: Verifies that lowered VIR modules contain exact instructions, section assignments, and symbol references via `// vir: <substr>` markers.
- **`programs/`**: Full Objective-C programs executed against real platform frameworks (Foundation, AppKit). Each program is compiled with `objv` and native `clang`, executed, and verified for identical output.

---

## Status

`objv` is an active, fully functional Objective-C compilation pipeline:

- **Lexer & Preprocessor**: Complete ISO phase 1–4 handling, `#import` idempotency, `@import`, variadic macros, token concatenation (`##`), stringification (`#`), and system header search.
- **Parser & AST**: Resilient parsing of C11 expressions, declarations, statements, Objective-C classes, categories, protocols, methods, ivars, properties, blocks, and attributes.
- **Semantic Analysis**: Full class hierarchies, protocol conformances, method lookup, property synthesis, lightweight generics, fast enumeration, and ARC ownership rules.
- **Runtime ABI & Lowering**: Lowers `objc_msgSend` dispatches, block closures, `@autoreleasepool`, `@try`/`@catch` exception landing pads, `.cxx_destruct` methods, and runtime metadata tables.
- **Backend & Linker**: Native Mach-O, ELF, and PE code generation and linking.

---

## Documentation

- [Objective-C Grammar Specification](docs/objc_grammar.md)
- [Vertex Intermediate Representation (VIR)](https://github.com/vertex-language/ir)

---

## License

MIT License. See [LICENSE](LICENSE) for details.
