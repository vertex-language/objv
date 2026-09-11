# internal/cli

`package cli` implements the `objv` command line. `cmd/objv` calls `Run` and
does nothing else.

```
objv build  [flags] [files...]   compile and link; with --emit, stop earlier
objv run    [flags] [file] [-- args...]
objv check  [flags] [files...]
objv ast    [flags] [file]
objv tokens [flags] [file]
objv env    [flags]
objv version
```

## It is a wrapper, and that is the rule

Everything about compiling Objective-C — the phases, the targets, the SDK, the
predefines, the link — is the `objv` package's. This package is what a
*command* adds to it: flags, where an artifact lands, standard input, the
caret under a diagnostic, and an exit code.

Nothing here decides anything a library caller would also have to decide. Two
copies of the pipeline is two places for the phases to drift apart, which is
the failure mode the phase model exists to prevent — so when a verb needs
something the library does not expose, the fix is to expose it, not to reach
around.

`Run` is the entire API. Everything else is unexported: the CLI is a consumer
of the library, never a library itself.

## What the command owns

Four things, and they are four things a library must not do.

**Standard input.** A pipe is the command line's. The library takes bytes; `-`
and a missing filename are what read them.

**The temporary directory `objv run` builds into.** A library that wrote a
file somewhere and deleted it later would be doing something its caller cannot
see, which is the one thing a compiler used from a build system must not do.

**`SOURCE_DATE_EPOCH`.** The CLI always supplies an epoch — the Unix zero when
the environment names none — so a build is deterministic with no flag saying
so. The library never reads it: a compiler that behaves one way in a terminal
and another in a test is a compiler nobody can trust.

**The target check.** `objv build -target pdp11-unix` says which flag fixes it
and lists what exists. That is the one thing a library cannot say and the one
thing a person at a terminal wants to read.

## Exit codes

gcc-shaped, so `objv check f.m && echo ok` means what it looks like.

| | |
| --- | --- |
| 0 | no error diagnostics |
| 1 | the input had errors |
| 2 | the invocation or I/O had errors |

`objv run` is the exception: it returns the *program's* exit code, because a
program that exits 1 did not fail to build.

## Diagnostics

Every diagnostic the library returns is already sited in a real file — phase
4's where it found them, and the later phases' mapped back out of the
preprocessed text — so nothing here decides *where*. What is left is
presentation:

```
NSString.h:491:25: error: unknown type name 'NSStringTransform'
    - (BOOL)applyTransform:(NSStringTransform)transform reverse:(BOOL)reverse
                            ^^^^^^^^^^^^^^^^^
    in file included from Foundation.h:24
    in file included from t.m:1
```

The caret is laid out in *raw* coordinates, so it lands on what the user wrote
even through trigraphs and line splices. The include chain earns its place in
an Objective-C compiler more than in a C one: one `#import
<Foundation/Foundation.h>` reaches some nine hundred headers, and a diagnostic
in one of them is unreadable without the path that got there.

## Where an artifact lands

`--emit mi`, `--emit vir` and `--emit obj` write one artifact per input.

- With `-o`, there is exactly one input and `-o` is where it goes. Two inputs
  and one `-o` is a mistake with one answer, so it says so rather than writing
  one file twice.
- Without one, a single text artifact goes to standard output — that is what a
  pipe expects — and everything else is named for its input, in the working
  directory. Where `cc -c` puts one, and not beside the source.
- `-o -` is for the text artifacts. An object file and an executable need a
  path.

## Tests

The library is tested where it lives. What is tested here is everything a
wrapper decides — where an artifact lands, what `-o` means with two inputs,
which exit code a verb returns, what a diagnostic looks like on a terminal —
and `Run` takes its writers as arguments precisely so it can be.

Most of them pass `-freestanding` and hermetic source, so the suite gives the
same answers on a machine with no Xcode. The two that link and run a binary
skip anywhere but arm64 macOS, because there the kernel is the judge.
