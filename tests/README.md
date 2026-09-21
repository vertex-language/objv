# tests

A ladder: `001`–`200`, one small thing per file, numbered in the order they
climb. `001`–`100` are plain C and `101`–`150` Objective-C; `151`–`180` go
back for the C the first pass left out, and `181`–`200` for the Objective-C
and Foundation.

Every file is built twice, once by objv and once by clang, then run, and
stdout and the exit status (or the signal that killed it) are compared.
Nothing here writes down an expected value: clang's answer is the oracle,
and a disagreement with it is a bug in objv by definition.

| | |
| --- | --- |
| 001–020 | arithmetic, one operator at a time: add, sub, mul, div, mod, shifts, bitwise, compares, logical |
| 021–040 | every integer width signed and unsigned, promotion, conversions, bool, float, double, NaN and infinity |
| 041–056 | control flow: if, loops, break/continue, goto, switch, recursion, statics and globals |
| 057–066 | pointers, arrays and strings |
| 067–080 | structs by value (8, 16, >16 bytes, HFA), layout, unions, enums, bit-fields |
| 081–100 | function pointers, varargs, the calling convention, the stack (VLAs, big frames, alignment), callee-saved registers |
| 101–127 | classes, ivars, properties, dealloc, dispatch, messages to nil, float and struct results, categories, protocols, literals |
| 128–135 | blocks, `@autoreleasepool`, exceptions, `@synchronized` |
| 136–150 | ARC one rule at a time: strong, weak, unsafe_unretained, method families, the return handshake, parameters, globals, loops, pool exits, ivars, CF bridging, cycles, returns with no pool |
| 151–166 | size_t, division by constants, `__int128`, bit and overflow builtins, ctype, flexible arrays, anonymous members, tagged unions, pointers to arrays, global tables, enums with fixed types |
| 167–180 | bit-field edges, math.h, HFA arrays and mixed structs, va_list forwarding, snprintf, setjmp, large constants, `?:` over structs, loop and switch exits, deep recursion |
| 181–190 | class properties, `+load`/`+initialize`, ivar layout, struct arguments, NSValue, description, isEqual/hash, copying, generics, block enumeration |
| 191–200 | `__block` objects, recursive blocks, struct blocks, `@finally` exits, `NSError **`, ARC struct members, `?:` objects, self-assignment, dispatch_once, JSON |

## Rules

- **One thing per file.** A failure should name what broke. When a test
  turns out to be asking two questions, split it.
- **No undefined behavior.** Signed overflow, out-of-range float-to-int and
  the like are left out: clang and objv are both allowed to answer anything.
- **Each run is capped** at 10 seconds and 1 MB of output, so a miscompiled
  loop fails its own test instead of the whole run.
- **Deterministic output only**: no clock, no addresses, no hash order, no
  threads, **no UI**. Dictionary keys are sorted before printing. Deallocs
  are printed or counted, never inferred from retain counts under ARC.
- **Every objv fix lands with the smallest numbered program that shows it.**

## Markers

On a line of their own:

```objc
// mode: arc          // or both; without it a file is built once, without ARC
// frameworks: CoreGraphics   // Foundation is always linked
// libraries: m
```

clang runs with `-ffp-contract=off` (objv doesn't fuse `a*b+c` yet) and
`-Werror=implicit-function-declaration`, so a missing `#include` can't be
hidden by clang inventing a declaration.

Run with `go test -run TestCorpus .` from the repository root, or a single
rung with `-run 'TestCorpus/113'`. arm64 macOS with clang only.
