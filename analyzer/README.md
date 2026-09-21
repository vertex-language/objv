# analyzer

`package analyzer` is the semantic front end: scopes and namespaces, the
class hierarchy, method and property resolution, expression typing, ARC, and
the constraint checks the parser deliberately deferred.

```
import "github.com/vertex-language/objv/analyzer"
```

```go
info, diags := analyzer.Check(unit, file, types.LP64(), analyzer.ARC)
```

The `Info` is never nil; diagnostics are sorted and each mistake is reported
once.

## Two passes, and why

```
pass 1   every @interface, @protocol, @class and category — names and links
pass 2   everything else, in written order
```

C needs no such thing: a name must be declared before it is used. Objective-C
does. A method may send a message to a class declared further down the file, a
category may extend a class the file has not reached, a protocol may be
adopted before it is declared. So the first pass exists only to make the
second possible.

Pass 1 registers **names and links only** — superclass, adopted protocols,
type parameters — and no member types. A member's type may name a typedef
further up the file that pass 1 has not processed, and nothing in the
hierarchy depends on it.

Within an `@implementation` the same problem recurs at a smaller scale, and
gets the same answer: every method signature is registered before any body is
read, so a method may call one defined below it. clang parses bodies last for
exactly this reason.

## Namespaces

C has three — ordinary identifiers, tags, labels. Objective-C adds three that
are **flat**: classes, protocols, selectors.

The flatness is the language's. `@class Forward;` inside an `@implementation`
names a class for the rest of the unit; a protocol is never scoped to
anything; and a selector is a name the whole program shares — two classes may
implement `count` and it is *one* selector, which is exactly why a message
send needs a receiver type to say which method it reaches.

A class exists as soon as it is named, and is completed in place, so every
pointer to it is the same class however it was reached. `@compatibility_alias`
enters the *same* entity under a second name rather than a copy.

Inside a method body the scope is not an ordinary one: `self` and `_cmd` are
declared, every visible instance variable of the class and its superclasses is
a bare name (§4.5), and the parameters sit on top. That is the only place in
the language where a declaration written elsewhere puts a name in scope.

## What it checks

**Sends** resolve against the receiver's static type — the class and its
superclasses, the protocols it adopts, the protocols a qualified `id` names.
A send that resolves is checked argument by argument; one that does not is
reported with the class that was searched, and a selector differing only in
case draws a "did you mean". A send to plain `id` is *not* an error — the
runtime resolves it — but a selector nothing in the unit declares draws a
warning, because that is a misspelling far more often than it is dynamism.

**`instancetype` follows the receiver.** `[[NSMutableString alloc] init]` is an
`NSMutableString`, not an `id`, and that is one substitution at each send.

**Type arguments are substituted.** `NSArray<NSString *> *a; a[0]` is an
`NSString *`, because `-objectAtIndexedSubscript:` is declared on
`NSArray<ObjectType>` and the receiver says what `ObjectType` is. The
substitution reaches into blocks too, which is what every enumeration method's
completion handler needs.

**Properties** bring an instance variable and two accessors with them, and
dot syntax resolves to the accessor. The modern runtime synthesizes by
default, so a property with no `@synthesize` and no `@dynamic` still gets
storage — unless the class wrote the accessors itself, which is what a
`readonly` property with a hand-written getter is.

**An implementation is checked against its promises**: the methods its own
interface declared, and the required methods of every protocol it adopts.
Both are *warnings*, because a missing method is a program that compiles and
fails when the selector is sent — which is what the language chose when it
made dispatch dynamic, and what clang says too.

**Visibility** (§4.5) is enforced: `@private` reaches the class that declared
it, `@protected` reaches its subclasses.

The Objective-C statements carry their own constraints: fast enumeration takes
an object pointer and an object collection, `@catch` takes an object pointer,
`@synchronized` takes an object, a bare `@throw` is valid only inside a
`@catch`, and a `@try` needs a `@catch` or a `@finally`.

## ARC is a mode

```go
analyzer.Check(unit, file, model, analyzer.ARC)
```

It is a flag because it is one in the language: `-fobjc-arc` decides it,
`__has_feature(objc_arc)` reports it, and a unit compiled without it is manual
reference counting all the way down.

What ARC decides here is **ownership**: `__strong` is the default for an
object variable, and a property's attribute *is* an ownership — `copy`,
`retain` and `strong` keep the object alive, `weak` does not,
`unsafe_unretained` keeps a pointer that may outlive what it points at. An
object property written `assign` draws a warning, because the program almost
certainly meant `weak`.

What ARC **refuses**: sending `retain`, `release`, `autorelease`,
`retainCount` or `dealloc` — the compiler is inserting those, and a program
that also sends them is double-counting — and an object pointer as a struct
member, which has no place to run a release.

What ARC **requires**: one of §6.5's three bridge casts wherever a conversion
crosses the boundary it manages. The diagnostic names all three, because
which one is right is the program's decision and not the compiler's:

```
cast between 'void*' and 'NSString*' requires a bridge cast under ARC:
__bridge to transfer nothing, __bridge_retained to hand ownership out,
__bridge_transfer to take it in
```

The retains and releases themselves are **lower's**, placed where ownership
changes. What this package owes it is the ownership.

## The tree is what was written

Property dot syntax stays a `MemberExpr` and object subscripting stays an
`IndexExpr`; what analysis adds is `Info.Props` and `Info.Sends`, which say
what each one resolved to. The rewrite into a message send is lower's, and a
diagnostic pointing at code the user did not write would be worse than none.

## `Info`

| | |
| --- | --- |
| `Types` | every declaring node and every expression, to its type |
| `Sends` | each message expression, to the method it resolved to — `nil` means "checked, receiver was `id`", which is not the same as absent |
| `Props` | each dot-syntax access, to the property it names |
| `Consts`, `Enums` | the constant expressions that were required to be constant, and every enumerator's value |
| `Classes`, `Protocols`, `Selectors` | what the unit declared and mentioned, in first-seen order — the order the runtime metadata is emitted in |

## Diagnostics

A nil type means "not known", and **nothing is reported about an operand whose
type is not known**. A checker that guesses produces diagnostics about correct
code, which is worse than silence: the user cannot fix it, and learns to
ignore the compiler.

One mistake is one diagnostic. An undeclared name is reported once per
translation unit — a misspelling inside a loop is one mistake however many
times it is written.

`types.Assignable` says *which* rule failed, so the message can too:

```
assigning 'NSString*' to 'NSMutableString*': the classes are unrelated;
'NSMutableString' is a subclass of 'NSString', so this narrows and needs a cast
```

## Tests

`analyzer_test.go` covers what `Info` records and what ARC decides.

Every program in [`tests`](../tests) is checked on its way to being built and
compared with clang's build of the same file.

## Dependencies

Imports [`ast`](../ast), [`token`](../token), [`types`](../types) and
[`parser`](../parser) — the last only in tests. It implements
`types.Resolver`, which is how construction asks what names mean.

Imported by `lower`, which reads `Info`, and by the `objv` package, which runs
it.
