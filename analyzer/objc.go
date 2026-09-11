package analyzer

import (
	"strings"

	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/token"
	"github.com/vertex-language/objv/types"
)

// ---- pass 1: the hierarchy ----

// declareObjC registers what a declaration says about the class and protocol
// namespaces, and nothing else.
//
// Only names and links: the superclass, the adopted protocols, the type
// parameters. Member types are not built here, because they may name a
// typedef further up the file that pass 1 has not reached — and because
// nothing in the hierarchy depends on them.
func (c *checker) declareObjC(d ast.Decl) {
	switch d := d.(type) {
	case *ast.ClassForwardDecl:
		for _, f := range d.Names {
			k := c.class(c.name(f.Name))
			c.declareTypeParams(k, f.TypeParams)
		}

	case *ast.ProtocolForwardDecl:
		for _, n := range d.Names {
			c.protocol(c.name(n)).Declared = true
		}

	case *ast.ClassInterfaceDecl:
		k := c.class(c.name(d.Name))
		c.declareTypeParams(k, d.TypeParams)
		if d.Super != nil {
			super := c.class(c.name(d.Super))
			if super == k {
				c.report(d.Super, "class '"+k.Name+"' cannot be its own superclass")
			} else {
				k.Super = super
			}
		}
		k.Protocols = append(k.Protocols, c.adopted(d.Protocols)...)

	case *ast.CategoryDecl:
		k := c.class(c.name(d.Class))
		k.Protocols = append(k.Protocols, c.adopted(d.Protocols)...)
		if d.Name != nil {
			k.Categories = append(k.Categories, c.name(d.Name))
		}

	case *ast.ProtocolDecl:
		p := c.protocol(c.name(d.Name))
		p.Declared = true
		p.Inherited = append(p.Inherited, c.adopted(d.Protocols)...)

	case *ast.CompatAliasDecl:
		// §4.4: the alias names the same class. It is the class, not a copy
		// of it, so everything that resolves through either resolves to one
		// entity.
		if d.Alias != nil && d.Class != nil {
			c.classes[c.name(d.Alias)] = c.class(c.name(d.Class))
		}
	}
}

func (c *checker) adopted(l *ast.ProtocolRefList) []*types.Protocol {
	if l == nil {
		return nil
	}
	var out []*types.Protocol
	for _, n := range l.Names {
		out = append(out, c.protocol(c.name(n)))
	}
	return out
}

// declareTypeParams records a generic class's parameters on the class.
//
// The bound is not built here — it may name a class pass 1 has not reached —
// so the parameters carry `id` until the @interface is read in order, which
// is where the bounds are resolved.
func (c *checker) declareTypeParams(k *types.Class, l *ast.TypeParamList) {
	if l == nil || len(k.TypeParams) > 0 {
		return
	}
	for _, p := range l.Params {
		k.TypeParams = append(k.TypeParams, &types.TypeParam{
			Name:     c.name(p.Name),
			Bound:    types.ID(),
			Variance: variance(p.VarianceKey),
			Owner:    k,
		})
	}
}

func variance(k token.Kind) types.Variance {
	switch k {
	case token.COVARIANT:
		return types.Covariant
	case token.CONTRAVARIANT:
		return types.Contravariant
	}
	return types.Invariant
}

// ---- pass 2: interfaces ----

func (c *checker) checkInterface(d *ast.ClassInterfaceDecl) {
	k := c.class(c.name(d.Name))
	if k.Complete {
		c.report(d.Name, "duplicate interface definition for class '"+k.Name+"'")
		return
	}
	k.Complete = true
	k.Root = d.Super == nil

	if d.Super != nil {
		if super := c.class(c.name(d.Super)); !super.Complete {
			// A superclass has to be a class this unit knows the shape of:
			// the subclass inherits its ivars and its methods, and a
			// forward declaration says neither.
			c.report(d.Super, "attempting to use the forward class '"+super.Name+
				"' as a superclass of '"+k.Name+"'")
		}
	} else if !c.hasAttr(d.Attrs, "objc_root_class") {
		c.warn(d.Name, "class '"+k.Name+"' defined without a superclass; "+
			"a root class needs __attribute__((objc_root_class))")
	}

	c.withTypeParams(k, d.TypeParams, func() {
		if d.Ivars != nil {
			c.addIvars(k, d.Ivars, types.VisProtected)
		}
		c.addMembers(k, nil, d.Members, k.Name)
	})
}

// withTypeParams runs f with the class's generic parameters in scope, and
// with their bounds resolved. §5.5's bound is a full TypeName and may name
// the class's own parameters, so it is built inside the scope it belongs to.
func (c *checker) withTypeParams(k *types.Class, l *ast.TypeParamList, f func()) {
	outer := c.typeParams
	if len(k.TypeParams) > 0 {
		c.typeParams = map[string]*types.TypeParam{}
		for name, p := range outer {
			c.typeParams[name] = p
		}
		for _, p := range k.TypeParams {
			c.typeParams[p.Name] = p
		}
		if l != nil {
			for i, p := range l.Params {
				if i < len(k.TypeParams) && p.Bound != nil {
					k.TypeParams[i].Bound = c.typeName(p.Bound)
				}
			}
		}
	}
	f()
	c.typeParams = outer
}

func (c *checker) checkCategory(d *ast.CategoryDecl) {
	k := c.class(c.name(d.Class))
	name := "a class extension"
	if d.Name != nil {
		name = "category '" + c.name(d.Name) + "'"
	}
	if !k.Complete {
		c.report(d.Class, "cannot declare "+name+" on the forward class '"+k.Name+"'")
		return
	}
	if d.Ivars != nil {
		if d.IsExtension() {
			// §4.2: only an extension may add instance variables, and they
			// are private — the extension is part of the implementation.
			c.addIvars(k, d.Ivars, types.VisPrivate)
		} else {
			c.report(d.Ivars, "a category may not declare instance variables; "+
				"a class extension may")
		}
	}
	owner := k.Name
	if d.Name != nil {
		owner = k.Name + "(" + c.name(d.Name) + ")"
	}
	c.withTypeParams(k, d.TypeParams, func() {
		c.addMembers(k, nil, d.Members, owner)
	})
}

func (c *checker) checkProtocol(d *ast.ProtocolDecl) {
	p := c.protocol(c.name(d.Name))
	if p.Complete {
		c.report(d.Name, "duplicate protocol definition for '"+p.Name+"'")
		return
	}
	p.Complete = true
	c.addMembers(nil, p, d.Members, p.Name)
}

// addMembers reads an interface's or protocol's member list. Exactly one of
// k and p is non-nil.
//
// §4.3's @required and @optional are markers in the list rather than a field
// on each member, so the state is carried across the walk — which is also
// how a marker written where §4.3 does not allow one is reported.
func (c *checker) addMembers(k *types.Class, p *types.Protocol, members []ast.Decl, owner string) {
	optional := false
	for _, m := range members {
		switch m := m.(type) {
		case *ast.RequirementDecl:
			if p == nil {
				c.report(m, "'"+m.Kind.String()+"' is meaningful only in a protocol")
				continue
			}
			optional = m.Kind == token.AT_OPTIONAL

		case *ast.MethodDecl:
			sig := c.methodSig(m, owner)
			if sig == nil {
				continue
			}
			sig.Optional = optional
			c.addMethod(k, p, sig, m)

		case *ast.PropertyDecl:
			c.addProperties(k, p, m, owner, optional)

		default:
			c.checkDecl(m, false)
		}
	}
}

// addMethod enters a method on a class or protocol, reporting a duplicate.
func (c *checker) addMethod(k *types.Class, p *types.Protocol, sig *types.Method, at ast.Node) {
	var list *[]*types.Method
	if k != nil {
		list = &k.Methods
	} else {
		list = &p.Methods
	}
	for i, prev := range *list {
		if prev.Sel != sig.Sel || prev.Class != sig.Class {
			continue
		}
		// An accessor a @property implied is not a declaration the program
		// made, so declaring it outright is not a redeclaration — it is the
		// same method, said in the second of the two ways §4.8 admits.
		// NSTimeZone declares `abbreviationDictionary` both ways in one
		// @interface, and so do a dozen more Foundation classes.
		if prev.FromProperty && !sig.FromProperty {
			(*list)[i] = sig
			c.selector(sig.Sel)
			return
		}
		if sig.FromProperty {
			return
		}
		// A category redeclaring a method the class already has is a
		// duplicate the runtime resolves by load order, which is to say
		// unpredictably.
		c.report(at, "duplicate declaration of method '"+methodName(sig)+"'")
		c.note(at, "previously declared in '"+prev.Owner+"'")
		return
	}
	*list = append(*list, sig)
	c.selector(sig.Sel)
}

func methodName(m *types.Method) string {
	if m.Class {
		return "+" + m.Sel
	}
	return "-" + m.Sel
}

// methodSig builds a method's signature from its declaration.
func (c *checker) methodSig(m *ast.MethodDecl, owner string) *types.Method {
	sig := &types.Method{
		Class:       m.IsClassMethod(),
		Owner:       owner,
		Variadic:    m.Ellipsis.IsValid(),
		Designated:  c.hasAttr(m.TailAttrs, "objc_designated_initializer") || c.hasAttr(m.Attrs, "objc_designated_initializer"),
		Unavailable: c.hasAttr(m.TailAttrs, "unavailable") || c.hasAttr(m.Attrs, "unavailable"),
	}
	c.inMethodRet = true
	sig.Ret = c.methodType(m.Type, types.ID())
	c.inMethodRet = false

	switch {
	case m.Sel != nil:
		sig.Sel = c.name(m.Sel)
	default:
		var pieces []string
		for _, part := range m.Parts {
			pieces = append(pieces, c.name(part.Sel))
			t := c.methodType(part.Type, types.ID())
			name := c.name(part.Name)
			sig.Params = append(sig.Params, types.Param{Name: name, Type: t})
		}
		sig.Sel = types.Selector(pieces, true)
	}
	// §4.7's MethodParameterSuffix: the C-style trailing parameters.
	for _, p := range m.Params {
		sp := types.BuildSpecs(c.unit, p.Specs, c)
		t, id := types.BuildDeclarator(c.unit, sp.Type, p.Decl, true, c)
		sig.Params = append(sig.Params, types.Param{Name: c.name(id), Type: types.AdjustParam(t)})
	}
	if sig.Sel == "" {
		return nil
	}
	c.info.Types[m] = sig.Ret
	return sig
}

// methodType builds §4.7's parenthesized type, defaulting to id.
//
// The default is the language's: a method whose type is omitted returns id,
// which is what `- (oneway)shutdown` and the pre-ANSI spelling both rely on.
func (c *checker) methodType(mt *ast.MethodType, def types.Type) types.Type {
	if mt == nil || mt.Type == nil {
		return def
	}
	return c.typeName(mt.Type)
}

// addIvars enters an instance-variable list on a class.
//
// §4.5's default visibility depends on where the list was written —
// @protected in an interface, @private in an implementation or extension —
// so the caller passes it and the markers in the list change it from there.
func (c *checker) addIvars(k *types.Class, l *ast.IvarList, def types.Visibility) {
	vis := def
	for _, item := range l.Items {
		switch item := item.(type) {
		case *ast.VisibilityDecl:
			switch item.Kind {
			case token.AT_PRIVATE:
				vis = types.VisPrivate
			case token.AT_PROTECTED:
				vis = types.VisProtected
			case token.AT_PUBLIC:
				vis = types.VisPublic
			case token.AT_PACKAGE:
				vis = types.VisPackage
			}

		case *ast.FieldDecl:
			sp := types.BuildSpecs(c.unit, item.Specs, c)
			for _, fd := range item.List {
				t, id := types.BuildDeclarator(c.unit, sp.Type, fd.Decl, false, c)
				iv := types.Ivar{Name: c.name(id), Type: c.ownership(t, false), Vis: vis}
				if fd.Colon.IsValid() {
					iv.BitField = true
					iv.Width, _ = c.requireConst(fd.Width, "bit-field width")
				}
				if iv.Name == "" {
					continue
				}
				if prev, owner := k.FindIvar(iv.Name); prev != nil {
					where := "class '" + owner.Name + "'"
					c.report(fd, "duplicate instance variable '"+iv.Name+"' in "+where)
					continue
				}
				// A flexible array member is the exception, as it is in a
				// struct: the last instance variable may be `T name[]`,
				// whose size is whatever was allocated past the object.
				// NSDecimal's _mantissa is one, and objc4 lays it out the
				// same way.
				if !types.Complete(t) && !flexibleArray(t, item, fd) {
					c.report(fd, "instance variable has incomplete type "+t.String())
				}
				c.info.Types[fd] = iv.Type
				k.Ivars = append(k.Ivars, iv)
			}

		default:
			c.checkDecl(item, false)
		}
	}
}

// addProperties enters the properties of one @property declaration.
func (c *checker) addProperties(k *types.Class, p *types.Protocol, d *ast.PropertyDecl,
	owner string, optional bool) {

	attrs, getter, setter := c.propertyAttrs(d)
	sp := types.BuildSpecs(c.unit, d.Specs, c)
	for _, decl := range d.List {
		t, id := types.BuildDeclarator(c.unit, sp.Type, decl, false, c)
		name := c.name(id)
		if name == "" {
			continue
		}
		prop := &types.Property{
			Name: name, Type: c.propertyOwnership(t, attrs, d), Attrs: attrs,
			Getter: getter, Setter: setter, Owner: owner,
		}
		if prop.Getter == "" {
			prop.Getter = name
		}
		if prop.Setter == "" {
			prop.Setter = "set" + capitalize(name) + ":"
		}
		if attrs&types.PropReadonly == 0 {
			c.selector(prop.Setter)
		}
		c.selector(prop.Getter)
		c.info.Types[decl] = prop.Type

		switch {
		case k != nil:
			if prev := findOwnProperty(k, name, attrs&types.PropClass != 0); prev != nil {
				c.report(decl, "duplicate property '"+name+"' in class '"+k.Name+"'")
				continue
			}
			k.Properties = append(k.Properties, prop)
			c.declareAccessors(k, prop, optional, decl)
		case p != nil:
			p.Properties = append(p.Properties, prop)
			c.declareProtocolAccessors(p, prop, optional)
		}
	}
}

// flexibleArray reports whether a member's incomplete type is the admitted
// one: an array with no bound, in the last declarator of the last member.
func flexibleArray(t types.Type, item *ast.FieldDecl, fd *ast.FieldDeclarator) bool {
	a, ok := types.Unqualify(t).(*types.Array)
	if !ok || a.Form != types.IncompleteArray {
		return false
	}
	return len(item.List) > 0 && item.List[len(item.List)-1] == fd
}

// findOwnProperty finds a property the class declares itself.
//
// A class property and an instance property may share a name, because they
// are reached through different selectors on different objects: NSDate has
// both `description` and a class `timeIntervalSinceReferenceDate`, and
// NSThread declares `isMainThread` twice for exactly this reason.
func findOwnProperty(k *types.Class, name string, class bool) *types.Property {
	for _, p := range k.Properties {
		if p.Name == name && p.Has(types.PropClass) == class {
			return p
		}
	}
	return nil
}

// declareAccessors adds the methods a property implies. A property is two
// methods and a variable, and the methods are what a message send and dot
// syntax both find — so they are declared here rather than invented at each
// use.
func (c *checker) declareAccessors(k *types.Class, p *types.Property, optional bool, at ast.Node) {
	class := p.Has(types.PropClass)
	if k.Lookup(p.Getter, class) == nil {
		k.Methods = append(k.Methods, &types.Method{
			Sel: p.Getter, Class: class, Ret: p.Type, Optional: optional,
			Owner: p.Owner, FromProperty: true,
		})
	}
	if !p.Has(types.PropReadonly) && k.Lookup(p.Setter, class) == nil {
		k.Methods = append(k.Methods, &types.Method{
			Sel: p.Setter, Class: class, Ret: types.Typ(types.Void),
			Params:   []types.Param{{Name: p.Name, Type: p.Type}},
			Optional: optional, Owner: p.Owner,
			FromProperty: true,
		})
	}
}

func (c *checker) declareProtocolAccessors(q *types.Protocol, p *types.Property, optional bool) {
	class := p.Has(types.PropClass)
	if q.Lookup(p.Getter, class) == nil {
		q.Methods = append(q.Methods, &types.Method{
			Sel: p.Getter, Class: class, Ret: p.Type, Optional: optional, Owner: p.Owner,
		})
	}
	if !p.Has(types.PropReadonly) && q.Lookup(p.Setter, class) == nil {
		q.Methods = append(q.Methods, &types.Method{
			Sel: p.Setter, Class: class, Ret: types.Typ(types.Void),
			Params:   []types.Param{{Name: p.Name, Type: p.Type}},
			Optional: optional, Owner: p.Owner,
		})
	}
}

// propertyAttrs folds §4.8's attribute list into a set, reporting the
// combinations that contradict each other.
func (c *checker) propertyAttrs(d *ast.PropertyDecl) (types.PropertyAttr, string, string) {
	var attrs types.PropertyAttr
	getter, setter := "", ""
	for _, a := range d.Attrs {
		switch a.Kind {
		case ast.PropReadonly:
			attrs |= types.PropReadonly
		case ast.PropReadwrite:
			attrs |= types.PropReadwrite
		case ast.PropAssign:
			attrs |= types.PropAssign
		case ast.PropRetain:
			attrs |= types.PropRetain
		case ast.PropCopy:
			attrs |= types.PropCopy
		case ast.PropStrong:
			attrs |= types.PropStrong
		case ast.PropWeak:
			attrs |= types.PropWeak
		case ast.PropUnsafeUnretained:
			attrs |= types.PropUnsafeUnretained
		case ast.PropAtomic:
			attrs |= types.PropAtomic
		case ast.PropNonatomic:
			attrs |= types.PropNonatomic
		case ast.PropClass:
			attrs |= types.PropClass
		case ast.PropDirect:
			attrs |= types.PropDirect
		case ast.PropNullable:
			attrs |= types.PropNullable
		case ast.PropNonnull:
			attrs |= types.PropNonnull
		case ast.PropNullResettable:
			attrs |= types.PropNullResettable
		case ast.PropNullUnspecified:
			attrs |= types.PropNullUnspecified
		case ast.PropGetter:
			attrs |= types.PropGetter
			getter = c.name(a.Sel)
			c.selector(getter)
		case ast.PropSetter:
			attrs |= types.PropSetter
			setter = c.name(a.Sel) + ":"
			c.selector(setter)
		}
	}
	// The ownership attributes name one behaviour each, and a property has
	// one.
	own := attrs & (types.PropAssign | types.PropRetain | types.PropCopy |
		types.PropStrong | types.PropWeak | types.PropUnsafeUnretained)
	if bits(uint32(own)) > 1 {
		c.report(d, "property attributes '"+ownNames(own)+"' are mutually exclusive")
	}
	if attrs&types.PropReadonly != 0 && attrs&types.PropReadwrite != 0 {
		c.report(d, "property cannot be both readonly and readwrite")
	}
	if attrs&types.PropAtomic != 0 && attrs&types.PropNonatomic != 0 {
		c.report(d, "property cannot be both atomic and nonatomic")
	}
	if attrs&types.PropReadonly != 0 && attrs&types.PropSetter != 0 {
		c.report(d, "a readonly property has no setter to name")
	}
	return attrs, getter, setter
}

func bits(n uint32) int {
	k := 0
	for ; n != 0; n &= n - 1 {
		k++
	}
	return k
}

func ownNames(a types.PropertyAttr) string {
	var out []string
	for _, e := range []struct {
		a types.PropertyAttr
		s string
	}{{types.PropAssign, "assign"}, {types.PropRetain, "retain"},
		{types.PropCopy, "copy"}, {types.PropStrong, "strong"},
		{types.PropWeak, "weak"}, {types.PropUnsafeUnretained, "unsafe_unretained"}} {
		if a&e.a != 0 {
			out = append(out, e.s)
		}
	}
	return strings.Join(out, ", ")
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	if s[0] >= 'a' && s[0] <= 'z' {
		return string(s[0]-'a'+'A') + s[1:]
	}
	return s
}

// hasAttr reports whether an attribute list carries one of that name, in
// either of §5.9's spellings: every attribute may be written with two
// leading and trailing underscores.
func (c *checker) hasAttr(attrs []*ast.Attr, name string) bool {
	for _, a := range attrs {
		if c.attrName(a) == name {
			return true
		}
	}
	return false
}
