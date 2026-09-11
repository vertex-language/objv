# lower

`package lower` turns one checked translation unit into one VIR module.

```
import "github.com/vertex-language/objv/lower"
```

```go
mod, diags := lower.Lower(file, tree, info, lower.Options{
    Name:         "counter",
    Target:       ir.AArch64MacOS,
    Model:        types.LP64(),
    ABI:          runtime.Darwin64(),
    Arch:         runtime.ARM64,
    SymbolPrefix: "_",
})
```

It is the end of the front end. Everything above it decided what the program
means; this decides nothing and only writes it down — which is why it reports
almost no diagnostics of its own, and why the ones it does report are either
"this construct is not implemented yet" or "internal:".

The module is **never nil**, even when diagnostics come back: a partial module
is what `--emit vir` on broken input should print. Diagnostics are sorted, and
a sticky builder failure inside `ir` surfaces as exactly one of them, since
every call after the first is a no-op and reporting each would be a cascade.

## Two passes

`declareFile` names everything at file scope, so that a body may call a
function declared below it and a method may reach the class it is defined in.
`defineFile` then walks the same declarations and fills them in.

The split is not stylistic. A symbol this unit both **defines and references**
has to be one symbol, and whoever asks for it first decides whether the module
defines the name or imports it. That went wrong three times — the class
object, the ivar offset variables, and ordinary functions — before every one
of them was moved into the first pass:

```
declareFile  →  @_OBJC_CLASS_$_Counter        defined, initializer empty
                @_OBJC_IVAR_$_Counter$_count  defined, offset written
                @_twice                        defined, body empty
defineFile   →  bodies, which reference all three
emitMetadata →  the initializers the first pass left blank
```

A class or ivar this unit does *not* implement is imported instead, by the
same lookup. `Options.SymbolPrefix` is applied on the way out, so a name in
this package is the C name and a name in the module is the object-file name.

One more thing the first pass decides is *whether to emit at all*. A `static`
function nothing in the unit mentions cannot be called from anywhere, and a
C99 inline definition provides no external definition (§6.7.4p7) — so both
are emitted only where they are used, which `useset.go` computes as a closure
from the unconditional roots. It is not an optimization: Apple's `<math.h>`
and `<objc/objc.h>` define inline functions in terms of builtins, and one
`#import <Foundation/Foundation.h>` brings in dozens. A declaration is not a
demand either: a prototype whose definition is not emitted gets no import,
and a declaration this package cannot give a VIR type is remembered and
reported at the use, if a use comes.

## Where the allocations go

The entry block holds `ptr.alloc` and nothing else, and the body goes in a
block of its own that entry branches to.

§19.6 admits `alloc` in the entry block only, and a frame slot is not always
wanted at the top of a function: a local declared after a loop, a block
literal built after one, a fast enumeration's state. By then the entry block
would have been terminated by the loop's first branch. Keeping it open until
the body is finished is what makes the two rules compatible, and it costs one
branch that every backend folds away.

## What a method is

A function of `self` and `_cmd`, and nothing else:

```objc
- (int)count { return _count; }
```

```
internal func @__i_Counter__count(%self ptr, %_cmd ptr) i32 {
@entry:
  %0 = ptr.getaddr @_OBJC_IVAR_$_Counter$_count
  %1 = i64.sload32 %0
  %2 = ptr.add %self, %1
  %3 = i32.load %2
  return %3
}
```

Two things in that are the whole Objective-C object model. The symbol is
mangled — `-[Counter count]` is not an identifier and a VIR symbol is — and
the instance variable's offset is **loaded**, not added as a constant, because
the runtime writes it when it realizes the class. That is the non-fragile ABI:
a framework may grow a member without every program that subclassed it having
to be rebuilt.

## What a send is

A call **through a pointer** to `objc_msgSend`, typed with the call site's own
signature:

```
%fp = ptr.getaddr @_objc_msgSend
%r  = callind %fp : @msgsig_ptr_ptr_i32_ri32(%recv, %sel, %arg)
```

One trampoline has as many signatures as there are methods: it forwards
whatever it was handed, and the caller and the method have to agree about the
registers. A direct call would fix one signature on the imported symbol and
mis-call every other method through it. clang solves this by casting the
function pointer at each call site; this names a func typedef per distinct
shape, and two methods of one shape share one declaration.

Which `objc_msgSend` is the target's question, and `runtime.Send` answers it —
the variants differ in how they forward a return value, and calling the wrong
one corrupts it rather than failing to link.

`super` does not send to the superclass. It sends through `objc_msgSendSuper2`
with a two-word structure of the receiver and the class the method was
compiled in, and the runtime finds the superclass itself — which is what lets
a category attached to the superclass afterwards still be found.

## The sugar that is a send

Three constructs look like C and are messages. All of them go through the same
`send`, and each evaluates its receiver exactly once:

| written | lowered to |
| --- | --- |
| `obj.name` | `[obj name]` |
| `obj.name = x` | `[obj setName:x]` |
| `obj.name += 1` | getter, `+`, setter |
| `a[i]` | `[a objectAtIndexedSubscript:i]` |
| `d[k] = v` | `[d setObject:v forKeyedSubscript:k]` |
| `@42` | `[NSNumber numberWithInt:42]` |
| `@[a, b]` | `[NSArray arrayWithObjects:buf count:2]` |

None of them is an lvalue: `&obj.name` is an error naming the getter, because
what dot syntax reads is not in memory.

`@"…"` is the exception. It is a **constant object in the image** — on Darwin
a CFString, whose isa is CoreFoundation's — because it has to work before any
class is realized. A literal with one non-ASCII character changes its
encoding, its section and its length field all at once.

## The metadata

None of it is reachable from any call. The runtime finds its work by walking
sections, which is why every list is emitted whether or not the program
mentions it and why the sections say `no_dead_strip`.

Every structure is declared as a **named VIR struct**, built from the field
lists in `runtime`:

```
internal type @objc_method_4 struct {
  entsize i32,
  count i32,
  entries [4]@objc_method
}
```

That is not decoration. `verify.Module` checks an initializer's shape against
its declared type, so a run of bytes with a list of pointers poured into it is
not a module — it is a module that happens to print. The corpus verifies every
file for exactly this reason.

A protocol is emitted by every image that mentions it and coalesced by the
linker down to one, so its symbols are weak and hidden: no single image owns a
protocol.

## Types

Two questions, answered separately.

`reg` is how a **value** is held — VIR has no i8 or i16 registers, so a narrow
integer lives in an i32 and `mem.go` says how much of it to store. `ftype` is
how an **object** is laid out, and a global needs one because its initializer
is checked against it. A record becomes a named struct whose every field
carries the offset `types.Model` computed, plus an explicit tail for the
padding `sizeof` counts — the layout VIR sees is the layout the analyzer
measured, not a second opinion arrived at independently.

## Blocks

A block literal becomes three things and sometimes five: a structure built
where the literal was written, a function holding its body, a descriptor the
runtime reads, and — when the literal captured something the runtime has to
keep alive — a copy helper and a dispose helper. `runtime/block.go` has the
layout, checked against what clang emits.

The structure *is* an object: its first word is an isa, which is why a block
can be sent `-copy` and put in an `NSArray`. Calling one is
`b->invoke(b, args…)` — the block passes itself as a hidden first argument,
and that is the only way the body reaches a capture.

A literal that captured nothing is a **global** block: nothing about it
differs between two executions of the statement that wrote it, so the whole
structure is a constant in `(__DATA,__const)` and the expression is its
address. One that captured something is a **stack** block, built into the
frame by stores; a program that wants it to outlive the frame says so, with
`Block_copy` or `-copy`.

Which variables are captured is not decided here. It is a question about C's
scopes — whether `n` in the body is the enclosing function's or one the block
declared — so the analyzer answers it, in `analyzer.Info.Captures`, in the
order the body first named each one, which is the order they are laid out in.
An instance variable inside a block captures `self`, because §4.5's `_count`
is `self->_count` there as anywhere else.

## Initializers

`{ 1, 2, 3 }` and the object it fills are different shapes, and §6.7.9p17 says
how to reconcile them: the initializers are consumed in order and the object
is descended into, brace by brace, until a scalar wants one. That walk is
written twice — `init.go` emits stores into an object that exists, and
`constinit.go` builds the value a static object *is* — because a file-scope
initializer is a constant expression and never runs.

A braced initializer zeroes what it does not mention (§6.7.9p21), which is one
`memset` rather than a store per hole — and is also what makes a partly
designated initializer correct without tracking which slots were filled.

## Not yet lowered

Each of these reports once, as an error, naming the construct rather than the
expression:

| | |
| --- | --- |
| `__block` variables | a block captures a copy; `__block` shares the variable, through a structure with a reference count of its own that both the function and the block reach it through |
| a block capturing a struct by value | a memcpy into the literal and one back out |
| `@try` / `@catch` / `@finally` | needs every call inside the region to become an `invoke` with an unwind edge. `@throw` is lowered; the rest is not |
| structs by value | a parameter, a return, or a message that returns one. Needs SysV and AAPCS classification, which is a package of its own |
| bit-fields | reading, writing, and initializing one |
| `@available` | needs the availability tables |
| inline assembly | `ir` has an asm form; nothing maps constraints onto it yet |

## Tests

`../tests/lower/` asks whether a file becomes the right IR. A file states what
it expects as substrings of the module text:

```objc
- (int)count { return _count; }
// vir: ptr.getaddr @_OBJC_IVAR_$_Counter$_count
// vir: i64.sload32
```

Substrings and not line numbers, because a lowering has no line to be on: one
statement becomes several blocks and one expression several instructions, and
a test anchored to a line would fail on every change to the *order* things are
emitted in rather than on a change to what is emitted.

Every file must also lower without a diagnostic and pass `verify.Module`. That
is the half of the contract no marker states, and it is the half that catches
the bugs.
