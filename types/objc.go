package types

import "strings"

// Objective-C type model (matching clang).
//
// An interface is an *Object, and `id` is an object pointer without a class named.
// Classes and protocols have pointer identity.

// Class is an Objective-C class (@interface).
// An incomplete class is forward-declared with @class.
type Class struct {
	Name     string
	Super    *Class
	Complete bool

	// Root marks a class with no superclass (__attribute__((objc_root_class)) or NSObject).
	Root bool

	Protocols  []*Protocol
	TypeParams []*TypeParam

	// SuperArgs records the type arguments specialized for the superclass.
	SuperArgs  []Type
	Ivars      []Ivar
	Methods    []*Method
	Properties []*Property

	// Categories records names of categories extending this class, in declaration order.
	Categories []string
}

// Protocol is an Objective-C protocol (@protocol).
type Protocol struct {
	Name string

	// Declared is true if forward-declared or defined; Complete is true if defined with a body.
	Declared   bool
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

// Object is an interface type (§5.4), optionally specialized and qualified by protocols.
// Base is nil for `id`. Meta is true for `Class`. Instancetype is true for instancetype.
// An Object always appears as a Pointer element.
type Object struct {
	Base         *Class
	Meta         bool
	Instancetype bool
	Args         []Type      // type arguments (§5.5)
	Protocols    []*Protocol // protocol qualifiers (§4.3)
}

func (*Object) Kind() Kind { return ObjectKind }

// Block is a block pointer type (§5.7).
type Block struct{ Sig *Func }

func (*Block) Kind() Kind { return BlockKind }

// Visibility is instance-variable visibility (§4.5).
type Visibility uint8

const (
	VisProtected Visibility = iota // default in class interface
	VisPrivate                     // default in implementation/extension/category
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
	Name        string
	Type        Type
	Vis         Visibility
	BitField    bool
	Width       int64
	Synthesized bool // compiler-synthesized for a property
}

// Method is a method's signature (§4.7).
type Method struct {
	Sel          string // full selector string including colons
	Class        bool   // true for class method (+)
	Ret          Type
	Params       []Param
	Variadic     bool
	Optional     bool   // protocol method under @optional
	FromProperty bool   // accessor synthesized or declared for a @property
	Designated   bool   // __attribute__((objc_designated_initializer))
	Unavailable  bool   // __attribute__((unavailable))
	Owner        string // enclosing class, category, or protocol
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

// PropertyAttr represents property attributes (§4.8) as a bitset.
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
	Name    string
	Type    Type
	Attrs   PropertyAttr
	Getter  string // getter selector
	Setter  string // setter selector
	Ivar    string // backing ivar name (empty for @dynamic)
	Dynamic bool
	Owner   string
}

// Has reports whether the property carries an attribute.
func (p *Property) Has(a PropertyAttr) bool { return p.Attrs&a != 0 }

// SelectorRecord is the tag of the struct SEL points at.
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

// AsObject returns the Object a pointer points at, or nil.
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

// IsObjCObject reports whether a value of t is an object pointer, block, or type parameter.
func IsObjCObject(t Type) bool {
	return IsObjectPointer(t) || IsBlock(t) || AsTypeParam(t) != nil
}

// IsID reports whether t is `id`.
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

// Conforms reports whether an object type satisfies a protocol.
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
