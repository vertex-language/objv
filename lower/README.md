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

### `__block`

A capture is a copy, which is what makes it const and a block cheap.
`__block` asks for the other thing: one variable, shared by the function and
by every block that captured it, and still shared after the block outlives
the frame.

It is arranged by moving the variable out of the frame into a structure of
its own — `runtime.BlockByref` — that the frame and the literal both point
at. Every access goes through that structure's `forwarding` field rather than
to it directly, and the indirection is the mechanism: on the stack,
forwarding points at the structure itself; when `_Block_copy` moves it to the
heap, the stack copy's forwarding is rewritten to the heap one, and both
frames go on reading one object.

```
    __block int n = 5;          n++ inside a block becomes
                                byref->forwarding->n += 1
 ┌──────────────┐
 │ isa      = 0 │
 │ forwarding ──┼──▶ itself, until _Block_copy says otherwise
 │ flags    = 0 │
 │ size    = 32 │
 │ n        = 5 │
 └──────────────┘
```

The structure is handed back to the runtime on every path out of the
function, which is what frees the heap copy if one was made. A `__block`
declared inside a loop is disposed once at the end rather than once per
iteration, which leaks a heap copy per iteration and nothing else.

## `@available`

§6.10's check asks about the machine the program is *running* on. An image
with a deployment target of macOS 11 may be launched on 12, and this is how
it finds out — which is why an API introduced after the deployment target can
be called at all.

Two of the three answers are constants. A clause naming this platform that
the deployment target already satisfies is true and nothing is emitted: the
linker records the deployment target in `LC_BUILD_VERSION`, so the comparison
was settled before the program ran. A list naming no platform this image is
for is true as well — that is what the trailing `*` means, and it is what
lets one source file carry checks for platforms it is not being built for.

Only a clause about this platform asking for something newer becomes a call,
and the call is `_availability_version_check`, which is libSystem's. clang
emits `__isPlatformVersionAtLeast` instead: compiler-rt's wrapper around the
same function, with a fallback that reads SystemVersion.plist on systems too
old to have it. objv links libSystem and not compiler-rt, and its Darwin
deployment floor is above that fallback's range.

## Automatic reference counting

ARC is not a garbage collector and not a rewrite. It is a set of rules about
where *ownership* changes, and every call `arc.go` places is at a point where
it does. The analyzer decided the ownership — every object variable has a
lifetime, most programs write none, and §5.6's default is `__strong` — and
this places the operations that keep it true.

Two facts do all the work.

The first is that an object rvalue is either **owned** — the expression
produced it at +1, and somebody has to release it — or **borrowed**, valid
only for as long as whatever holds it holds it. Which one is decided by the
selector's *name*, and the naming convention is normative: `alloc`, `copy`,
`init`, `mutableCopy` and `new` return an object the caller owns, and every
other method returns one it does not. `runtime.FamilyOf` is that rule, corners
included — `newlineCharacterSet` is not in the `new` family, because the word
is "newline".

The second is that an owned value nobody takes has to be released at the end
of the full expression. So every owned value is registered as it is produced,
and a context that *takes* ownership — initializing a `__strong` variable,
assigning to one, returning from a method that returns +1 — takes it back off
the register instead. What is left at the end of the statement is what nothing
wanted.

That is the whole mechanism. `[[Box alloc] init]` registers alloc's +1, `init`
consumes its receiver and registers its own, and the declaration it
initializes takes that one — so the object is retained once, by the variable,
and released once, when the variable goes out of scope.

Three things follow that are easy to miss and fatal to omit:

- `self = [super init]` **takes** the +1 rather than replacing one, and
  `return self` hands back that same +1. Retaining it again leaves a count
  nothing brings down; releasing it frees the object being initialized.
- A class with `__strong` or `__weak` instance variables owes a
  `.cxx_destruct`, and the class's flag word has to say so — objc4 checks
  `RO_HAS_CXX_STRUCTORS` before it looks the selector up, so a destructor
  with the bit clear is a method nothing calls.
- A user-written `-dealloc` ends with `[super dealloc]`, which ARC inserts
  and forbids the program to write. Without it the override never reaches
  `object_dispose`, `.cxx_destruct` never runs, and everything the object
  holds leaks while the program looks like it worked.

`__weak` is four runtime calls and no stores: the runtime keeps a side table
of every weak reference to an object and walks it during dealloc, which is
what makes the reference go to nil. `loadFrom` and `storeTo` are the choke
points — a weak reference is never read by loading it.

Retaining a *block* is `objc_retainBlock` and not `objc_retain`, which is not
a name: a block literal lives in the frame that wrote it, and the call is what
copies it to the heap first. Everything that escapes — a block returned, a
block stored in a `__strong` variable or a `copy` property — goes through it.

`__block` is where the two mechanisms meet, and where the order matters. The
structure a `__block` variable lives in is reached through its `forwarding`
field, and that field's value *changes* under an assignment whose right-hand
side copies a block: `fact = ^{ … fact … }` retains the block, retaining a
block copies it to the heap, and a block that captured this variable takes
the structure with it. So the address is taken after the retain and not
before it — see `refreshByref` and `initByref` — and the payload is nil
before anything reads it, since a store into a `__strong` location releases
what it replaced and there would otherwise be a stack pattern there.

What is not here: the optimizations clang applies to pairs it can prove
redundant, which cost instructions and not correctness; and the release of a
`__strong` local left by a `goto` out of its scope, which leaks.

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

## The accessors nobody wrote

§4.8: a property with neither `@synthesize` nor `@dynamic` still has an
instance variable and a pair of accessors. The analyzer creates the
*declarations*, so that a send to one typechecks; this package owes the
bodies, and a class that does not emit them publishes selectors the runtime
cannot find — `unrecognized selector`, at run time, for a program the
compiler accepted.

Most bodies are one load or one store through the ivar's offset. The
interesting ones are a call, and which call is the whole of what a property's
attributes mean:

| | |
| --- | --- |
| nonatomic, assign | a load and a store through the offset |
| nonatomic, copy | `objc_setProperty_nonatomic_copy` |
| atomic, retain | `objc_getProperty`, `objc_setProperty_atomic` |

An atomic *scalar* is stored directly all the same: a word-sized store is
already indivisible, and clang emits the same thing. Only an object needs the
runtime, because reading a pointer and retaining it have to happen without a
setter running in between, and the runtime owns that lock.

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

## Bit-fields

§6.7.2.1p13 gives a bit-field no address, which is the whole of why it needs
a file of its own. Every other member is read by computing an address and
loading through it; a bit-field is read by loading the *allocation unit* it
shares with its neighbours and shifting the bits out, and written by loading
that unit, replacing the bits, and storing it back — a read-modify-write, for
what the source wrote as an assignment.

Where the bits are is not decided here. §6.7.2.1p11 leaves the allocation
implementation-defined and the two answers in circulation disagree about
ordinary structs, so the rule is the target's — `types.Model.MSBitfields` —
and one walk computes both the record's size and each field's placement.
`Model.BitPlaces` is that walk answering the second question, so the offsets
emitted and the size reported cannot drift apart.

The VIR type describes the *bytes*, not the fields: one array per merged
range of bit-field bytes, alongside the ordinary members and sorted with
them. The range is what the bits occupy rather than what the unit covers,
because a unit may overlap an ordinary member — `struct { char c; int b : 3; }`
opens a four-byte unit at zero and puts b in the byte after c — and two
fields of a struct type may not overlap.

The sign matters on the way out and not on the way in. A signed field of six
bits holds -32..31, so reading one shifts left to put its top bit in the sign
position and arithmetic-shifts back down; writing either only has to keep the
bits that fit.

## Structs by value

No calling convention passes a struct in a register, and what each does
instead is a classification: AAPCS64 asks whether the aggregate is
homogeneous, then whether it is sixteen bytes or less, and only then falls
back to the caller's copy by reference; SysV sorts each eightbyte into
INTEGER, SSE or MEMORY. Neither answer is derivable from the other.

None of it is here. VIR states the *question* in the signature — `byval` on a
pointer parameter says the bytes it points at are the argument, `sret` on the
first says the callee writes its result through it — and the backend answers
it per target. What this package owes is the rest: an aggregate argument has
to be a *copy*, because the callee owns its parameter and may assign to it,
and an aggregate result's storage is the caller's, allocated before the call.

A send carries the same two attributes on the trampoline's type, with the
hidden pointer in front of the receiver — which is where `objc_msgSend_stret`
wants it on x86-64 and where X8 puts it on AArch64. `runtime.Send` picks
which trampoline, because on AArch64 there is no `_stret` variant at all.

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

## Exceptions

`@try` is a region whose calls carry a second edge: every call inside one is an
`invoke` (§G3) to the region's landing pad, and the pad is a switch over the
selector the personality routine hands it. Each arm is a `@catch` body with
`objc_begin_catch` and `objc_end_catch` around it, and the clause a pad
declares is the type-info the personality matched against — `OBJC_EHTYPE_$_X`
for a class, `OBJC_EHTYPE_id` for `@catch (id)`, and no type-info at all for
`@catch (...)`.

`@finally` is the hard part, because it runs on every way out — falling off
the end, returning through, breaking out of a loop that crosses it, and
unwinding past — and has to be one copy of the block rather than one per exit.
So it is a block all of them reach, with a slot saying where to go afterwards
and a `br_table` over the codes; a `return` parks its value in a second slot,
because the return itself happens on the other side.

The unwinding exit is clang's rather than C++'s. The pad's last clause is a
catch-all, so the personality stops at this frame, `objc_begin_catch` takes the
object, the block runs, and `objc_exception_rethrow` puts it back on its way —
and that rethrow is an ordinary call, so a `@try` enclosing this one sees it as
an `invoke`. A `cleanup` clause and a `resume` would be the C++ spelling and
would be wrong twice over: `resume` hands control to the unwinder, which steps
past this frame entirely, and a block every exit reaches is dominated by no pad,
which is what §19.5 requires of `resume`'s operand.

`@synchronized` and `@autoreleasepool` are the same machinery without the
switch: each is a region whose pad unlocks or drains and rethrows, because a
lock a thrown exception left held is a deadlock rather than a leak. Both read
what they owe out of a frame slot rather than out of the value they were given
— a pad is reached only by an unwind edge, so the block the lock was taken in
does not dominate it.

One thing ARC does not yet do in a @try: a `__strong` local is not released on the
path that unwinds past its scope. The releases are emitted on the paths out
that lowering can see, and an unwind edge is not one of them — so an exception
crossing a scope leaks what that scope held.

## __func__

§6.4.2.2's predefined identifier is a *declaration* and not a macro — `static
const char __func__[] = "name"` at the top of every function body — so it is a
read-only array like any string literal, and the three spellings gcc gives it
share one. The name is clang's: `plain` for a function, `-[Class selector]`
for a method, `main_block_invoke` for a block, because a program that prints
`__func__` is comparing it with something.

`__PRETTY_FUNCTION__` is that same string here and a rendered signature under
clang — `void plain(void)`. Spelling a C declaration back out is a printer
this package does not have and would have to agree with character for
character to be worth anything, so it is the one place the two compilers
differ on purpose.

## Not yet lowered

Each of these reports once, as an error, naming the construct rather than the
expression:

| | |
| --- | --- |
| a struct in a variadic argument | legal C, but there is no declared parameter to hang `byval` on, so nothing states how it travels |
| a `goto` out of a `@try` that has a `@finally` | the destination table holds any number of exits and a `break` reaches one through it, but a `goto`'s label may not be lowered yet, so how many `@finally` blocks stand between here and it is not known where the `goto` stands |
| a bit-field instance variable | the runtime writes an ivar's offset in bytes, so packing several into one word means agreeing with clang about which bits each gets — a second layout question with the non-fragile ABI on the other side |
| inline assembly | `ir` has an asm form; nothing maps constraints onto it yet |

## Tests

`../tests/eval/` asks whether a file becomes the right IR. A file states what
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
