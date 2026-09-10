# runtime

`package runtime` describes the Objective-C runtime ABI: the symbols a
translation unit defines and references, the sections its metadata is placed
in, the layout of the structures the runtime walks, and the type encodings it
reads out of them.

```
import "github.com/vertex-language/objv/runtime"
```

It is **data and rules, not emission**. Nothing here writes an object file or
builds IR — `lower` does that, and it does it by asking this package what each
thing is called, how big it is, and what goes in it. The ABI is a contract
with a runtime nobody here controls, and a contract stated in one place can be
read, tested and corrected without touching a code generator.

Everything is the modern (non-fragile) runtime's, which is the only one objv
emits for.

## Where the answers came from

Every constant, layout and string in this package was taken from clang:

```console
$ xcrun clang -S -fobjc-arc t.m -o -      # the metadata
$ printf("%s", @encode(T))                # the encodings
```

The tests carry those answers verbatim, and `seam_test.go` closes the loop: it
runs a class through objv's **own** parser and analyzer and checks that the
types the front end built encode to the same strings clang emitted for the
same source. The unit tests say this package agrees with clang about a type
built by hand; that one says the compiler does.

## The non-fragile ABI in one symbol

```go
runtime.IvarOffsetSymbol("Cache", "_count")   // OBJC_IVAR_$_Cache._count
```

An instance variable's offset is not a constant in the instruction stream but
a **global the runtime writes** when it realizes the class. Every access loads
it and adds it. That is the whole of the non-fragile ABI: `class_ro_t` carries
the `instanceStart` the compiler believed, the runtime compares it against the
superclass's real size and slides every ivar by the difference, and a
framework that grew a member is a number that differs rather than every
program that subclassed it having to be rebuilt.

Symbols are written **without** the platform's leading underscore. Mach-O adds
one when the object writer emits the symbol, exactly as it does for a C
function, and a name that carried it here would be wrong on ELF.

Three prefixes carry meaning:

| | |
| --- | --- |
| `OBJC_CLASS_$_` | a class, which the linker resolves across images |
| `_OBJC_$_` | metadata private to this image |
| `l_OBJC_` | a label the assembler keeps and the linker does not |

A method's symbol is spelled the way the language spells the method —
`-[Cache setObject:forKey:]` — which is not a C identifier, cannot collide
with one, and is why a crash report is readable.

## Sections are an interface

```go
abi.Name(runtime.SecClassList)   // __DATA,__objc_classlist,regular,no_dead_strip
```

objc4 finds a class not by looking for a symbol but by walking
`__objc_classlist` in every loaded image. That is why every list is
`no_dead_strip`: a class nothing references is still a class the runtime must
register, because a message may name it at run time. The other attributes are
load-bearing too — `coalesced` on the protocol list, because every image that
mentions a protocol defines it and exactly one copy must survive;
`literal_pointers` on the selector references and `cstring_literals` on the
strings, which is what merges them across the link.

ELF and COFF have no segments and no attributes, so the names are the bare
ones libobjc2 looks for.

## Layouts are field lists

```go
abi.SizeOf(runtime.ClassRO)                    // 72
abi.OffsetOf(runtime.ClassRO, "baseMethodList") // 32, true
abi.EntSize(runtime.Method)                     // 24
```

A structure is described as an ordered list of `Field`s rather than as a Go
struct, because what `lower` needs is not a value — it never holds one — but
the order and width of what it writes. A field list is also what can be read
against objc4's headers, which is the only way to know any of it is right.

`class_ro_t` has four bytes of explicit padding on a 64-bit target. It is in
the list because a structure written without it is one field short from there
on, and because the runtime reads no field there — which is exactly the sort
of thing that has to be *stated* rather than derived.

A method, ivar or property list begins with `entsize` and `count`, and the
runtime iterates by **adding entsize** rather than by `sizeof`, so a list
written by a newer compiler with wider entries stays walkable. A protocol list
is different — a pointer-width count and a null terminator — and that is not
a mistake to correct.

`protocol_t` and `category_t` each write their **own size** into themselves,
which is the runtime's version check: an older runtime reads the fields it
knows and stops.

## Encodings

Three forms, for three jobs:

| | |
| --- | --- |
| `Encode` | what `@encode` gives a program: the shape of a value |
| `EncodeExtended` | the same with class names kept — `@"NSString"` — which is what an ivar and a property carry |
| `MethodTypes` | a signature with its frame layout, which `NSInvocation` and the forwarding machinery read |

The alphabet is objc4's and it is not consistent with anything else in the
compiler, so each irregularity is a case rather than a derivation:

```
char *          *        and not ^c
const char *    r*       the const goes in front of the pointer
const int       i        a top-level const is dropped
volatile int *  ^i       volatile is not in the alphabet at all
long            q        by width on LP64, not by spelling
SEL             :        a pointer to struct objc_selector, by its tag
void (^)(int)   @?       a block is an object whose class is not named
int (*)(char)   ^?       a function pointer names no signature
```

A struct containing a pointer to itself has to stop somewhere, and the
runtime's rule is that a pointer to a struct **already being written** is the
tag alone: `{Node=i^{Node}}`. A struct nested by value still expands.

A method's frame is the sum of the aligned argument sizes and is **not**
rounded up at the end; a struct returned indirectly does not appear in it.
Neither follows from the calling convention, and both are visible in what
clang emits:

```
- (int)feed:(int)n           i20@0:8i16
- (void)take:(struct Big)b   v56@0:8{Big=ddddd}16
- (struct Big)big            {Big=ddddd}16@0:8
```

## Property attributes

```go
runtime.PropertyAttributes(runtime.PropertyDesc{
    Type: `@"NSString"`, Copy: true, Nonatomic: true, Ivar: "_name",
})                                              // T@"NSString",C,N,V_name
```

The order is the runtime's: type, readonly, ownership, dynamic, atomicity,
getter, setter, backing variable. `assign` and `unsafe_unretained` write
*nothing* — the absence of an ownership letter is what they are — and so does
`atomic`.

## Dispatch

```go
abi.Send(runtime.AMD64, ret, model, super)   // objc_msgSend_stret
```

There is more than one `objc_msgSend` because the calling convention has more
than one shape for a return value, and the trampoline has to know which one it
is forwarding. Calling the wrong one corrupts the return value, so this is a
correctness question, not an optimization.

- **arm64 has no `_stret` at all**: the indirect result register is separate
  from the argument registers, so the ordinary trampoline forwards it without
  knowing it is there.
- **x86-64** needs `_stret` for a struct the classifier sends to memory, and
  `_fpret` only for `long double`.
- **i386** returns every struct indirectly and every float on the x87 stack.

`objc_msgSendSuper2` is not a version: it takes the class the method was
compiled in and finds the superclass itself, which is what lets a category
attached to the superclass afterwards still be found.

## Flags

`ClassFlags` builds `class_ro_t.flags`. The two ARC-shaped bits go together:
`RO_HAS_CXX_STRUCTORS` tells the runtime to look for `.cxx_destruct` — which
in Objective-C means the class has ivars to release — and
`RO_HAS_CXX_DTOR_ONLY` tells it not to look for a constructor.

## Dependencies

Imports [`types`](../types) and nothing else in the tree. It does **not**
import `ir`: what this package knows is what the runtime requires, and how
that becomes a global with an initializer is `lower`'s business.

Imported by `lower`, which emits the metadata, and by `runtime`'s own tests,
which reach through `parser` and `analyzer` to check the seam.
