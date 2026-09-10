package types

import "strings"

// The Objective-C type model, which is clang's.
//
// An interface is a type — *Object — and what a program calls "an NSString"
// is a pointer to one. `id` is that pointer with no class named, which is
// also what <objc/objc.h> says it is. The two-level shape is not ceremony:
// `__strong NSString *__weak *` has an ownership qualifier at each level, and
// a model that folded the pointer into the class could not say which is
// which.
//
// Classes and protocols have identity, on the same terms records do: two
// *Class values name the same class iff they are the same pointer. That is
// what makes `@class NSString;` followed by `@interface NSString` one class
// rather than two, and what lets a subclass test be a walk up a chain.

// Class is an Objective-C class: an @interface, whatever it was reached by.
//
// It is an entity and not a Type — a class has no Kind, and nothing can hold
// a value of one. What a program writes as a type is an *Object naming it,
// behind a pointer.
//
// A class named only by @class is incomplete — the name exists, and nothing
// else about it does — which is exactly enough to declare a pointer to one
// and not enough to send it a message.
type Class struct {
	Name     string
	Super    *Class
	Complete bool

	// Root marks a class with no superclass:
	// __attribute__((objc_root_class)), or NSObject itself. A root class is
	// where the metaclass chain closes, so the runtime metadata needs to
	// know which one it is.
	Root bool

	Protocols  []*Protocol
	TypeParams []*TypeParam
	Ivars      []Ivar
	Methods    []*Method
	Properties []*Property

	// Categories records the names of the categories that extended this
	// class, in the order they were read. A category's members are folded
	// into Methods and Properties — that is what a category does — and this
	// is what a diagnostic uses to say which one a duplicate came from.
	Categories []string
}

// Protocol is an Objective-C protocol. Like a class it has identity, and
// like a class it may be forward-declared and incomplete.
type Protocol struct {
	Name       string
	Complete   bool
	Inherited  []*Protocol
	Methods    []*Method
	Properties []*Property
}

// TypeParam is one of §5.5's lightweight generic parameters.
//
// Bound is what the parameter is constrained to, and is `id` when none was
// written. The parameter is erased: a value of the type is an object pointer
// and nothing else, and the bound is what the front end checks against and
// what the analyzer substitutes when a specialization names an argument.
type TypeParam struct {
	Name     string
	Bound    Type
	Variance Variance
	Owner    *Class
}

func (*TypeParam) Kind() Kind { return TypeParamKind }

func (p *TypeParam) String() string { return p.Name }

// Variance is §5.5's __covariant / __contravariant.
type Variance uint8

const (
	Invariant Variance = iota
	Covariant
	Contravariant
)

func (v Variance) String() string {
	switch v {
	case Covariant:
		return "__covariant"
	case Contravariant:
		return "__contravariant"
	}
	return ""
}

// Object is §5.4's ObjectTypeSpecifier: an interface type, optionally
// specialized and optionally qualified by protocols.
//
// Base is nil for `id`. Meta is `Class`, the type of a class object.
// Instancetype is the placeholder a method's return type may be written as,
// which the analyzer resolves to the receiver's class at each send.
//
// A value never has this type — an interface cannot be declared by value —
// so an Object always appears as a Pointer's element. The declaration that
// tries otherwise is a diagnostic the analyzer gives, and it needs this type
// to exist in order to give it.
type Object struct {
	Base         *Class
	Meta         bool
	Instancetype bool
	Args         []Type      // §5.5's type arguments
	Protocols    []*Protocol // §4.3's protocol qualifiers
}

func (*Object) Kind() Kind { return ObjectKind }

// Block is a block pointer type (§5.7): a pointer to a closure with the
// given signature.
//
// It is not a Pointer to a Func. A block pointer may not be dereferenced,
// it carries captured state, and it is an object — a block assigns to `id`
// and is retained and released like any other. A model that read it as a
// function pointer would have to make three exceptions to say so.
type Block struct{ Sig *Func }

func (*Block) Kind() Kind { return BlockKind }

// Visibility is §4.5's instance-variable visibility.
type Visibility uint8

const (
	// VisProtected is the default in a class interface; VisPrivate is the
	// default in an implementation, an extension and a category. §4.5 makes
	// the default depend on where the variable was written, so the parser's
	// caller decides it and this records what it decided.
	VisProtected Visibility = iota
	VisPrivate
	VisPublic
	VisPackage
)

func (v Visibility) String() string {
	switch v {
	case VisPrivate:
		return "@private"
	case VisPublic:
		return "@public"
	case VisPackage:
		return "@package"
	}
	return "@protected"
}

// Ivar is one instance variable.
type Ivar struct {
	Name     string
	Type     Type
	Vis      Visibility
	BitField bool
	Width    int64

	// Synthesized marks an ivar the compiler created for a property rather
	// than one the program wrote. It matters to a diagnostic — a name
	// collision with one of these is the property's fault, not the
	// programmer's — and to the runtime metadata, which emits both alike.
	Synthesized bool
}

// Method is a method's signature (§4.7).
//
// Sel is the selector as one string, colons included: `setObject:forKey:`,
// `init`, `a::`. It is built once, here, because everything downstream keys
// on it — the runtime's dispatch table, the analyzer's lookup, the
// diagnostic that says a method is not found — and a selector assembled
// twice is a selector that can differ.
type Method struct {
	Sel      string
	Class    bool // written with '+'
	Ret      Type
	Params   []Param
	Variadic bool

	// Optional marks a protocol method under @optional. A class is not
	// required to implement it, and a send to a type qualified by the
	// protocol may find nothing at run time.
	Optional bool

	// FromProperty marks an accessor a @property implied rather than one
	// the program wrote. §4.8 says a property declares its getter and its
	// setter, and a class may also declare either outright — Apple does it
	// to give an accessor an availability the property does not have — so
	// the two are the same method declared twice and not a redeclaration.
	FromProperty bool

	// Designated is __attribute__((objc_designated_initializer)), which
	// decides which initializers a subclass must override and which ones
	// may only chain. Unavailable is __attribute__((unavailable)), which is
	// how a class refuses an inherited initializer.
	Designated  bool
	Unavailable bool

	// Owner names the class, category or protocol the method was declared
	// in, for the diagnostic that has to say where the other one is.
	Owner string
}

// String renders a method the way the language writes one, which is what a
// diagnostic about a selector should say.
func (m *Method) String() string {
	sign := "-"
	if m.Class {
		sign = "+"
	}
	var b strings.Builder
	b.WriteString(sign + " (" + m.Ret.String() + ")")
	if len(m.Params) == 0 {
		b.WriteString(m.Sel)
		return b.String()
	}
	parts := strings.SplitAfter(m.Sel, ":")
	for i, p := range m.Params {
		if i < len(parts) {
			b.WriteString(parts[i])
		}
		b.WriteString("(" + p.Type.String() + ")" + p.Name)
		if i < len(m.Params)-1 {
			b.WriteString(" ")
		}
	}
	if m.Variadic {
		b.WriteString(", ...")
	}
	return b.String()
}

// PropertyAttr is the attribute set of §4.8, as a bitset: the attributes are
// independent, and a property carries several.
type PropertyAttr uint32

const (
	PropReadonly PropertyAttr = 1 << iota
	PropReadwrite
	PropAssign
	PropRetain
	PropCopy
	PropStrong
	PropWeak
	PropUnsafeUnretained
	PropAtomic
	PropNonatomic
	PropClass
	PropDirect
	PropNullable
	PropNonnull
	PropNullResettable
	PropNullUnspecified
	PropGetter
	PropSetter
)

// Property is a declared property (§4.8).
type Property struct {
	Name  string
	Type  Type
	Attrs PropertyAttr

	// Getter and Setter are the selectors the property is accessed
	// through — the ones `getter=` and `setter=` named, or the default
	// `name` and `setName:`. They are stored rather than derived because a
	// property with an explicit getter has no other record of it, and
	// because dot syntax resolves to exactly these.
	Getter string
	Setter string

	// Ivar is the instance variable backing the property, named by
	// @synthesize or defaulted to _name. It is empty for a @dynamic
	// property, which promises the accessors exist and provides none.
	Ivar    string
	Dynamic bool

	Owner string
}

// Has reports whether the property carries an attribute.
func (p *Property) Has(a PropertyAttr) bool { return p.Attrs&a != 0 }

// SelectorRecord is the tag of the structure SEL points at.
//
// <objc/objc.h> declares `typedef struct objc_selector *SEL;`, so a selector
// is a pointer to an incomplete record and nothing about its shape says what
// it is. The tag is what says it — the header has said so since 1988, no
// program may define another structure by that name, and the runtime's own
// type encoding gives it a letter of its own. Recognizing the tag here is
// what lets everything downstream tell a SEL from any other pointer.
const SelectorRecord = "objc_selector"

// NewSelector returns the type SEL names.
func NewSelector() Type {
	return &Pointer{Elem: &Record{Name: SelectorRecord}}
}

// IsSelector reports whether t is SEL.
func IsSelector(t Type) bool {
	p, ok := Unqualify(t).(*Pointer)
	if !ok {
		return false
	}
	r, ok := Unqualify(p.Elem).(*Record)
	return ok && r.Name == SelectorRecord && !r.Complete
}

// ---- constructing object types ----

// NewObject returns the pointer type a program writes as `Base *`, or as
// `id` when base is nil.
func NewObject(base *Class, protocols ...*Protocol) Type {
	return &Pointer{Elem: &Object{Base: base, Protocols: protocols}}
}

// ID is `id`: a pointer to an object of no particular class.
func ID() Type { return &Pointer{Elem: &Object{}} }

// ClassObject is `Class`: a pointer to a class object.
func ClassObject() Type { return &Pointer{Elem: &Object{Meta: true}} }

// Instancetype is the placeholder §5.4 admits as a method's return type.
func Instancetype() Type { return &Pointer{Elem: &Object{Instancetype: true}} }

// ---- predicates ----

// AsObject returns the Object a pointer points at, or nil. It is the test
// for "is this an object pointer", and it is the one every Objective-C rule
// starts with.
func AsObject(t Type) *Object {
	p, ok := Unqualify(t).(*Pointer)
	if !ok {
		return nil
	}
	o, _ := Unqualify(p.Elem).(*Object)
	return o
}

// IsObjectPointer reports whether t is a pointer to an interface type.
func IsObjectPointer(t Type) bool { return AsObject(t) != nil }

// AsBlock returns t's block type, or nil.
func AsBlock(t Type) *Block {
	b, _ := Unqualify(t).(*Block)
	return b
}

// IsBlock reports whether t is a block pointer.
func IsBlock(t Type) bool { return Unqualify(t).Kind() == BlockKind }

// IsObjCObject reports whether a value of t is something the runtime
// retains and releases: an object pointer, a block, or a generic type
// parameter, which is erased to an object pointer and behaves as one
// everywhere a value of it is used.
func IsObjCObject(t Type) bool {
	return IsObjectPointer(t) || IsBlock(t) || AsTypeParam(t) != nil
}

// IsID reports whether t is `id` — an object pointer naming no class,
// carrying no protocols.
func IsID(t Type) bool {
	o := AsObject(t)
	return o != nil && o.Base == nil && !o.Meta && len(o.Protocols) == 0
}

// IsClassType reports whether t is `Class`.
func IsClassType(t Type) bool {
	o := AsObject(t)
	return o != nil && o.Meta
}

// AsTypeParam returns t's type parameter, or nil.
func AsTypeParam(t Type) *TypeParam {
	p, _ := Unqualify(t).(*TypeParam)
	return p
}

// ---- class and protocol relations ----

// IsSubclassOf reports whether c is a, or descends from, super.
func (c *Class) IsSubclassOf(super *Class) bool {
	for k := c; k != nil; k = k.Super {
		if k == super {
			return true
		}
	}
	return false
}

// Conforms reports whether the class or one of its superclasses adopts p,
// directly or through a protocol that inherits it.
func (c *Class) Conforms(p *Protocol) bool {
	for k := c; k != nil; k = k.Super {
		if conformsAny(k.Protocols, p) {
			return true
		}
	}
	return false
}

// Conforms reports whether the protocol is p or inherits it.
func (q *Protocol) Conforms(p *Protocol) bool {
	if q == p {
		return true
	}
	return conformsAny(q.Inherited, p)
}

func conformsAny(list []*Protocol, p *Protocol) bool {
	for _, q := range list {
		if q.Conforms(p) {
			return true
		}
	}
	return false
}

// Conforms reports whether an object type satisfies a protocol: because its
// class adopts it, or because the type was qualified by it.
//
// `id<NSCopying>` conforms to NSCopying with no class involved at all, which
// is the whole point of a qualified id: the protocol is the only thing known
// about the value.
func (o *Object) Conforms(p *Protocol) bool {
	if conformsAny(o.Protocols, p) {
		return true
	}
	return o.Base != nil && o.Base.Conforms(p)
}

// Lookup returns the method with the given selector, searching the class and
// then its superclasses. class selects between the two method families: a
// class method and an instance method may share a selector and are different
// methods.
func (c *Class) Lookup(sel string, class bool) *Method {
	for k := c; k != nil; k = k.Super {
		for _, m := range k.Methods {
			if m.Sel == sel && m.Class == class {
				return m
			}
		}
		// A class conforms to protocols, and a protocol's methods are
		// declared for every class that adopts it.
		for _, p := range k.Protocols {
			if m := p.Lookup(sel, class); m != nil {
				return m
			}
		}
	}
	return nil
}

// Lookup returns the method the protocol declares, searching the protocols
// it inherits.
func (q *Protocol) Lookup(sel string, class bool) *Method {
	for _, m := range q.Methods {
		if m.Sel == sel && m.Class == class {
			return m
		}
	}
	for _, p := range q.Inherited {
		if m := p.Lookup(sel, class); m != nil {
			return m
		}
	}
	return nil
}

// FindProperty returns the property of that name, searching superclasses and
// adopted protocols — which is what dot syntax resolves through.
func (c *Class) FindProperty(name string) *Property {
	for k := c; k != nil; k = k.Super {
		for _, p := range k.Properties {
			if p.Name == name {
				return p
			}
		}
		for _, proto := range k.Protocols {
			if p := proto.FindProperty(name); p != nil {
				return p
			}
		}
	}
	return nil
}

// FindProperty returns the property the protocol declares, searching the
// protocols it inherits.
func (q *Protocol) FindProperty(name string) *Property {
	for _, p := range q.Properties {
		if p.Name == name {
			return p
		}
	}
	for _, proto := range q.Inherited {
		if p := proto.FindProperty(name); p != nil {
			return p
		}
	}
	return nil
}

// FindIvar returns the instance variable of that name, searching
// superclasses — non-fragile ivars are still inherited.
func (c *Class) FindIvar(name string) (*Ivar, *Class) {
	for k := c; k != nil; k = k.Super {
		for i := range k.Ivars {
			if k.Ivars[i].Name == name {
				return &k.Ivars[i], k
			}
		}
	}
	return nil, nil
}

// ---- printing ----

func (c *Class) String() string {
	if c.Name == "" {
		return "<class>"
	}
	return c.Name
}

func (q *Protocol) String() string {
	if q.Name == "" {
		return "<protocol>"
	}
	return q.Name
}

func (o *Object) String() string {
	var b strings.Builder
	switch {
	case o.Instancetype:
		b.WriteString("instancetype")
	case o.Meta:
		b.WriteString("Class")
	case o.Base == nil:
		b.WriteString("id")
	default:
		b.WriteString(o.Base.Name)
	}
	if len(o.Args) > 0 {
		b.WriteByte('<')
		for i, a := range o.Args {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(a.String())
		}
		b.WriteByte('>')
	}
	if len(o.Protocols) > 0 {
		b.WriteByte('<')
		for i, p := range o.Protocols {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(p.Name)
		}
		b.WriteByte('>')
	}
	return b.String()
}

func (b *Block) String() string {
	if b.Sig == nil {
		return "void(^)()"
	}
	var s strings.Builder
	s.WriteString(b.Sig.Ret.String())
	s.WriteString("(^)(")
	if b.Sig.Proto && len(b.Sig.Params) == 0 {
		s.WriteString("void")
	}
	for i, p := range b.Sig.Params {
		if i > 0 {
			s.WriteString(", ")
		}
		s.WriteString(p.Type.String())
	}
	if b.Sig.Variadic {
		s.WriteString(", ...")
	}
	s.WriteByte(')')
	return s.String()
}
