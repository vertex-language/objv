# types

`package types` represents Objective-C types and constructs them from
declaration specifiers and declarators.

```
import "github.com/vertex-language/objv/types"
```

Types are trees; records, enums, classes and protocols additionally have
**identity** — two `*Record` values are the same type iff they are the same
pointer, and two `*Class` values name the same class on the same terms.
Constraint checking the parser deliberately deferred — specifier multisets,
qualifier placement, `static`/`*` in array declarators, function and array
derivation rules — happens here, during construction, reported through the
`Resolver`.

## The `Type` tree

```go
type Type interface {
	Kind() Kind
	String() string
}
```

| Type | Shape |
| --- | --- |
| `*Basic` | A builtin arithmetic type or `void`. Use `Typ(k)` for the canonical singleton of each kind. |
| `*Qualified` | Qualifiers wrapped around another type. **Never nests.** |
| `*Pointer` | Pointer-to-`Elem`. |
| `*Array` | Array-of-`Elem`, one of four `ArrayForm`s. |
| `*Func` | A function type. `Proto` is false for `f()`, where parameters are unspecified — as opposed to `f(void)`, a prototype with zero. |
| `*Record` | A struct or union. Identity, like `*Enum`. |
| `*Enum` | An enumerated type. `Fixed` records §5.8's `enum E : T`. |
| `*Object` | An **interface type** (§5.4): a class, `id`, `Class`, `instancetype`, with type arguments and protocol qualifiers. |
| `*Block` | A block pointer (§5.7). |
| `*TypeParam` | One of §5.5's lightweight generic parameters. |

## Objective-C is clang's model, not a simpler one

An interface is a type, and what a program calls "an NSString" is a **pointer
to one**:

```go
types.NewObject(nsstring)   // NSString *  →  *Pointer{ *Object{Base: NSString} }
types.ID()                  // id          →  *Pointer{ *Object{} }
types.ClassObject()         // Class       →  *Pointer{ *Object{Meta: true} }
```

Folding `NSString *` into one atomic thing would work until the first

```objc
__strong NSString *__weak *cell;
```

where there is an ownership qualifier at each level and a one-level model
cannot say which is which. The two-level shape is also what makes `id` honest:
it is a pointer whose spelling has no star, exactly as `<objc/objc.h>` defines
it, which is why `String()` prints `id` and `NSString*` from the same
`*Pointer` node.

A `*Class` is **not** a `Type`. It has no `Kind`, and nothing can hold a value
of one — it is the entity a type names. That is also the mechanism behind the
diagnostic for `NSString value;`: the specifier builds an `*Object`, the
declarator adds no pointer, and `Complete` says an interface type cannot be a
declared object.

A `*Block` is not a pointer to a `*Func`. A block pointer cannot be
dereferenced, it carries captured state, and it is an *object* — it assigns to
`id` and the runtime retains and releases it. Reading it as a function pointer
would need three exceptions to say so.

### Qualifiers, at three levels

```go
Qualify(t, QConst)                  // const, volatile, restrict, _Atomic, __kindof
WithLifetime(t, LifeWeak)           // §5.6's ARC ownership: one of five
WithNullability(t, NullNullable)    // §5.6's nullability: one of four
```

C's qualifiers are a bitset; ownership and nullability are enumerations,
because a type has one owner and one nullability, not a set of them. All three
ride on the same `*Qualified` wrapper and none of them nests — `Qualify` merges,
`WithLifetime` and `WithNullability` replace.

`__kindof` is a qualifier and not a flag on the object, because that is what it
does: it changes what may be assigned, not what the type is. It is written
before the class name and belongs on the pointer, so construction moves it
there — `__kindof NSString *` and `NSString *__kindof` are the same type, as
they are in clang.

## Assignment

`Assignable(dst, src, nullConst)` is C11 §6.5.16.1's constraint list plus
Objective-C's, and it returns *which* rule failed so the analyzer can say so:

| | |
| --- | --- |
| `AssignOK` | the constraints are met |
| `AssignPointerMismatch` | two pointers, to incompatible types — or two blocks with different signatures |
| `AssignDiscardsQuals` | the pointee types agree but the target drops a qualifier |
| `AssignIntPointer` | a pointer and an integer that is not a null pointer constant |
| `AssignObjCUnrelated` | two object pointers with no class relation, or the relation the wrong way round |
| `AssignObjCProtocol` | the classes are fine; the source is not known to conform to a protocol the destination requires |
| `AssignObjCTypeArgs` | the same generic class, specialized differently |

The rules are the language's. `id` converts to and from every object pointer;
a subclass converts to its superclass and not back; a protocol qualifier is a
promise the source has to keep; a block is an object and assigns to `id`.

**Type arguments are invariant** by default. `NSArray<NSMutableString *> *`
does not assign to `NSArray<NSString *> *`, because the array is writable and a
covariant rule would let a caller put an `NSString` into an array the callee
believes holds `NSMutableString`s. §5.5's `__covariant` is how a class opts
out, and `isVariantArg` is where that opt-out is honoured.

**`__kindof` admits the downcast** it exists for: `__kindof NSString *`
assigns to `NSMutableString *` with no cast.

### ARC is not decided here

```go
types.Bridge(dst, src)   // BridgeNone | BridgeNeeded
```

Whether a conversion crosses the boundary ARC manages is a fact about the two
types. Whether crossing it *without* one of §6.5's bridge keywords is an error
depends on whether ARC is on, which is a compilation flag — the analyzer's to
know, not this package's. So `Assignable` lets an object pointer and a `void *`
convert, and `Bridge` says that a keyword is what makes it explicit.

## Classes, protocols, methods

```go
c.IsSubclassOf(super)          // a walk up the chain
c.Conforms(protocol)           // directly, inherited, or through a superclass
c.Lookup("setObject:forKey:", false)   // the class, its superclasses, its protocols
c.FindProperty("name")         // what dot syntax resolves through
c.FindIvar("_count")           // non-fragile ivars are still inherited
```

A `*Method` carries the selector as **one string**, colons included:
`setObject:forKey:`, `init`, `a::`. It is assembled once, by `Selector`,
because everything downstream keys on it — the runtime's dispatch table, method
lookup, the diagnostic that says a method was not found — and a selector
assembled twice is a selector that can differ.

A `*Property` stores its getter and setter *selectors* rather than deriving
them, because a property with `getter=isHidden` has no other record of it and
dot syntax resolves to exactly those.

## Construction

```go
sp := types.BuildSpecs(unit, decl.Specs, resolver)          // a specifier list
t, id := types.BuildDeclarator(unit, sp.Type, d, false, r)  // through a declarator
```

`Resolver` is what construction needs from its caller: `Typedef`, `Tag`,
`Object`, `Eval`, `TypeOf`, and `Report`. The analyzer implements it; the test
resolver in `build_test.go` is the smallest thing that does.

`Resolver.Object` returns **what the specifier names**, and that is not always
the same shape: a class name is an interface type, because the source writes
the star and the declarator builds the pointer, while `id`, `Class` and
`instancetype` are already pointers.

Declarators derive **inside-out**, which is why the block case is short: by the
time the `^` is reached, the parameter list has already built the function type
the block points at.

`Spec` carries the non-type facts the specifiers held: storage class,
`__block`, `inline`, `_Noreturn`, `_Alignas` nodes, attributes, and `Auto` for
`__auto_type`, whose type is its initializer's and so cannot be known here.

## Layout

```go
m := types.LP64()
m.Sizeof(t)            // false for incomplete types, functions, VLAs, interfaces
m.Alignof(t)
m.Offsetof(record, "field")
```

`Model` is a target's type model, and layout is a pure function of it —
nothing here probes a host. Every Objective-C addition is a pointer: an object
pointer and a block pointer are both `SizePtr`, which is why none of them
appears in the model. What a class costs is the *runtime's* question:
instance variables are non-fragile, so their offsets are decided when the
program loads.

An interface type has no size, which is the honest answer and the one that
produces the right diagnostic.

Bit-fields pack into consecutive allocation units of their declared type;
`Record.MemberAlign` states `__attribute__((packed))` and `#pragma pack` once,
so that this layout and lower's cannot drift.

## Dependencies

Imports [`ast`](../ast) and [`token`](../token) — construction reads
declarators, and reports at nodes. Nothing else in the tree.

Imported by `analyzer`, which resolves the names construction asks about, and
by `lower`, which needs the same answers about size and conversion that the
analyzer used.
