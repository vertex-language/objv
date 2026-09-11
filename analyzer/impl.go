package analyzer

import (
	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/token"
	"github.com/vertex-language/objv/types"
)

// @implementation: what a class actually provides, checked against what its
// interface, its categories and its protocols promised.

func (c *checker) checkClassImpl(d *ast.ClassImplDecl) {
	k := c.class(c.name(d.Name))
	if !k.Complete {
		// §4.1: an implementation implements an interface. Without one
		// there is nothing that says what the class is, what it inherits,
		// or what it must provide.
		c.report(d.Name, "cannot find interface declaration for '"+k.Name+"'")
		return
	}
	if d.Super != nil {
		if super := c.class(c.name(d.Super)); super != k.Super {
			c.report(d.Super, "superclass '"+c.name(d.Super)+
				"' does not match the interface, which says '"+superName(k)+"'")
		}
	}
	if d.Ivars != nil {
		// §4.5: instance variables declared in an implementation are
		// private to it, which is what the default here says.
		c.addIvars(k, d.Ivars, types.VisPrivate)
	}
	c.implement(d.Name, k, d.Members, k.Name, true)
}

func superName(k *types.Class) string {
	if k.Super == nil {
		return "no superclass"
	}
	return k.Super.Name
}

func (c *checker) checkCategoryImpl(d *ast.CategoryImplDecl) {
	k := c.class(c.name(d.Class))
	if !k.Complete {
		c.report(d.Class, "cannot find interface declaration for '"+k.Name+"'")
		return
	}
	owner := k.Name + "(" + c.name(d.Name) + ")"
	c.implement(d.Name, k, d.Members, owner, false)
}

// implement checks an implementation's members.
//
// The method signatures are registered before any body is read. A method may
// send a message to one defined further down the same @implementation —
// clang parses bodies last for exactly this reason — and a method defined
// here but not declared in the interface is still a method of the class.
func (c *checker) implement(at ast.Node, k *types.Class, members []ast.Decl, owner string, whole bool) {
	c.withTypeParams(k, nil, func() {
		defined := map[string]*ast.MethodDecl{}

		for _, m := range members {
			md, ok := m.(*ast.MethodDecl)
			if !ok {
				continue
			}
			sig := c.methodSig(md, owner)
			if sig == nil {
				continue
			}
			key := methodName(sig)
			if prev, dup := defined[key]; dup {
				c.report(md, "duplicate definition of method '"+key+"'")
				c.note(prev, "previously defined here")
				continue
			}
			defined[key] = md
			// A method the interface declared is that method; one it did
			// not is added, which is how a private helper is written.
			if existing := k.Lookup(sig.Sel, sig.Class); existing != nil {
				c.checkOverride(md, existing, sig)
			} else {
				k.Methods = append(k.Methods, sig)
				c.selector(sig.Sel)
			}
		}

		// @synthesize and @dynamic, then the properties nobody mentioned,
		// which the modern runtime synthesizes on its own.
		c.propertyImpls(k, members, defined)
		if whole {
			c.autoSynthesize(k, members, defined)
		}

		for _, m := range members {
			switch m := m.(type) {
			case *ast.MethodDecl:
				c.checkMethodBody(k, m, owner)
			case *ast.PropertyImplDecl:
				// Handled above.
			case *ast.PropertyDecl:
				// §4.6 admits one in an implementation; it declares a
				// property whose accessors this class provides.
				// A property in an @implementation is another place a
				// readonly one is widened, so it redeclares like a
				// category's rather than colliding.
				c.addProperties(k, nil, m, owner, false, true)
			default:
				c.checkDecl(m, false)
			}
		}

		if whole {
			c.checkPromises(at, k, members)
		}
	})
}

// checkOverride reports a definition whose signature disagrees with the
// declaration it implements. A method is dispatched by selector, so two
// signatures for one selector is one method with two meanings.
func (c *checker) checkOverride(at ast.Node, declared, defined *types.Method) {
	if !types.Compatible(declared.Ret, defined.Ret) &&
		!(types.IsObjCObject(declared.Ret) && types.IsObjCObject(defined.Ret)) {
		c.report(at, "'"+defined.Sel+"' returns "+defined.Ret.String()+
			", but '"+declared.Owner+"' declares it as returning "+declared.Ret.String())
		return
	}
	if len(declared.Params) != len(defined.Params) {
		return // a different selector; nothing to compare
	}
	for i := range declared.Params {
		a, b := declared.Params[i].Type, defined.Params[i].Type
		if types.Compatible(a, b) || (types.IsObjCObject(a) && types.IsObjCObject(b)) {
			continue
		}
		c.report(at, "argument "+itoa(i+1)+" of '"+defined.Sel+"' is "+b.String()+
			", but '"+declared.Owner+"' declares it as "+a.String())
		return
	}
}

// propertyImpls applies @synthesize and @dynamic (§4.8).
func (c *checker) propertyImpls(k *types.Class, members []ast.Decl, defined map[string]*ast.MethodDecl) {
	for _, m := range members {
		pi, ok := m.(*ast.PropertyImplDecl)
		if !ok {
			continue
		}
		dynamic := pi.Kind == token.AT_DYNAMIC
		for _, item := range pi.Items {
			name := c.name(item.Name)
			p := k.FindProperty(name)
			if p == nil {
				c.report(item, "no property named '"+name+"' to "+
					map[bool]string{true: "@dynamic", false: "@synthesize"}[dynamic])
				continue
			}
			if dynamic {
				// @dynamic promises the accessors exist at run time and
				// provides none, which is what a Core Data property is.
				p.Dynamic = true
				continue
			}
			ivar := "_" + name
			if item.Ivar != nil {
				ivar = c.name(item.Ivar)
			}
			c.synthesize(k, p, ivar, item, defined)
		}
	}
}

// autoSynthesize creates the storage and accessors for every property the
// implementation did not mention.
//
// The modern runtime synthesizes by default, so a property with no
// @synthesize and no @dynamic still has an ivar and accessors — unless the
// program wrote the accessors itself, which is what a readonly property with
// a hand-written getter is.
func (c *checker) autoSynthesize(k *types.Class, members []ast.Decl, defined map[string]*ast.MethodDecl) {
	mentioned := map[string]bool{}
	for _, m := range members {
		if pi, ok := m.(*ast.PropertyImplDecl); ok {
			for _, item := range pi.Items {
				mentioned[c.name(item.Name)] = true
			}
		}
	}
	for _, p := range k.Properties {
		if mentioned[p.Name] || p.Dynamic || p.Has(types.PropClass) {
			continue
		}
		// A property whose accessors the class wrote itself needs no
		// storage: nothing would read the ivar.
		hasGetter := defined["-"+p.Getter] != nil
		hasSetter := defined["-"+p.Setter] != nil
		if hasGetter && (p.Has(types.PropReadonly) || hasSetter) {
			continue
		}
		c.synthesize(k, p, "_"+p.Name, nil, defined)
	}
}

// synthesize gives a property its instance variable and its accessors.
func (c *checker) synthesize(k *types.Class, p *types.Property, ivar string,
	at ast.Node, defined map[string]*ast.MethodDecl) {

	if p.Ivar != "" {
		if at != nil {
			c.report(at, "property '"+p.Name+"' is already synthesized")
		}
		return
	}
	if iv, owner := k.FindIvar(ivar); iv != nil {
		// A property may be backed by an instance variable the class
		// declared, which is what `@synthesize name = _name;` usually names.
		if !types.Compatible(iv.Type, p.Type) &&
			!(types.IsObjCObject(iv.Type) && types.IsObjCObject(p.Type)) {
			c.report(at, "property '"+p.Name+"' is "+p.Type.String()+
				" but the instance variable '"+ivar+"' in '"+owner.Name+"' is "+iv.Type.String())
		}
	} else {
		k.Ivars = append(k.Ivars, types.Ivar{
			Name: ivar, Type: p.Type, Vis: types.VisPrivate, Synthesized: true,
		})
	}
	p.Ivar = ivar

	// The accessors the class did not write itself.
	if defined["-"+p.Getter] == nil && k.Lookup(p.Getter, false) == nil {
		k.Methods = append(k.Methods, &types.Method{
			Sel: p.Getter, Ret: p.Type, Owner: k.Name,
		})
	}
	if !p.Has(types.PropReadonly) && defined["-"+p.Setter] == nil && k.Lookup(p.Setter, false) == nil {
		k.Methods = append(k.Methods, &types.Method{
			Sel: p.Setter, Ret: types.Typ(types.Void), Owner: k.Name,
			Params: []types.Param{{Name: p.Name, Type: p.Type}},
		})
	}
}

// checkPromises reports what the class said it would provide and did not:
// the methods its own interface declared, and the required methods of every
// protocol it adopts.
//
// Both are warnings, not errors. A missing method is a program that compiles
// and fails at run time when the selector is sent — which is what the
// language chose when it made dispatch dynamic — and clang says the same.
func (c *checker) checkPromises(at ast.Node, k *types.Class, members []ast.Decl) {
	implemented := map[string]bool{}
	for _, m := range members {
		if md, ok := m.(*ast.MethodDecl); ok && md.IsDefinition() {
			if sig := c.methodSig(md, k.Name); sig != nil {
				implemented[methodName(sig)] = true
			}
		}
	}
	// A synthesized property's accessors are implemented by the compiler.
	for _, p := range k.Properties {
		if p.Ivar != "" || p.Dynamic {
			implemented["-"+p.Getter] = true
			implemented["-"+p.Setter] = true
		}
	}

	for _, m := range k.Methods {
		if m.Owner != k.Name || implemented[methodName(m)] {
			continue
		}
		if at != nil {
			c.warn(at, "method '"+methodName(m)+"' declared by '"+k.Name+
				"' is not implemented")
		}
	}
	for _, p := range k.Protocols {
		c.checkProtocolPromises(k, p, implemented, at)
	}
}

func (c *checker) checkProtocolPromises(k *types.Class, p *types.Protocol,
	implemented map[string]bool, at ast.Node) {

	for _, m := range p.Methods {
		if m.Optional || implemented[methodName(m)] {
			continue
		}
		// A superclass may have implemented it; the protocol asks the
		// class, not this @implementation.
		if k.Super != nil && k.Super.Lookup(m.Sel, m.Class) != nil {
			continue
		}
		if at != nil {
			c.warn(at, "method '"+methodName(m)+"' required by protocol '"+p.Name+
				"' is not implemented by '"+k.Name+"'")
		}
	}
	for _, inherited := range p.Inherited {
		c.checkProtocolPromises(k, inherited, implemented, at)
	}
}

// checkMethodBody checks one method definition.
//
// The scope a method body runs in is not an ordinary one: self and _cmd are
// declared, every instance variable of the class and its superclasses is a
// name, and the parameters are on top of those. §4.5 is what makes an ivar a
// bare name, and it is the only place in the language where a declaration
// somewhere else puts a name in scope without being written.
func (c *checker) checkMethodBody(k *types.Class, m *ast.MethodDecl, owner string) {
	if !m.IsDefinition() {
		return
	}
	sig := c.methodSig(m, owner)
	if sig == nil {
		return
	}

	c.push()
	prevSelf, prevMeth := c.self, c.meth
	c.self, c.meth = k, sig

	self := types.NewObject(k)
	if sig.Class {
		self = c.classObjectType(k)
	}
	c.declareName("self", &symbol{kind: symObject, typ: self, node: m})
	if s := c.lookup("SEL"); s != nil && s.kind == symTypedef {
		c.declareName("_cmd", &symbol{kind: symObject, typ: s.typ, node: m})
	} else {
		c.declareName("_cmd", &symbol{kind: symObject, typ: types.ID(), node: m})
	}
	if !sig.Class {
		c.declareIvars(k, m)
	}
	for i, part := range m.Parts {
		if part.Name == nil || i >= len(sig.Params) {
			continue
		}
		c.declare(part.Name, &symbol{kind: symObject, typ: sig.Params[i].Type, node: part})
	}
	for _, p := range m.Params {
		sp := types.BuildSpecs(c.unit, p.Specs, c)
		t, id := types.BuildDeclarator(c.unit, sp.Type, p.Decl, true, c)
		if id != nil {
			c.declare(id, &symbol{kind: symObject, typ: types.AdjustParam(t), node: p})
		}
	}
	c.declareFuncNameText(m, methodName(sig))

	prevLabels, prevGotos, prevRet := c.labels, c.gotos, c.fnRet
	c.labels, c.gotos, c.fnRet = map[string]*ast.LabeledStmt{}, nil, sig.Ret
	for _, kr := range m.KR {
		c.checkDecl(kr, false)
	}
	c.checkStmt(m.Body, false)
	c.checkLabels()
	c.labels, c.gotos, c.fnRet = prevLabels, prevGotos, prevRet

	c.self, c.meth = prevSelf, prevMeth
	c.pop()
}

// declareIvars brings §4.5's instance variables into a method's scope as
// bare names.
func (c *checker) declareIvars(k *types.Class, at ast.Node) {
	// Superclass first, so that a subclass's own variable shadows an
	// inherited one of the same name rather than the other way round.
	var chain []*types.Class
	for x := k; x != nil; x = x.Super {
		chain = append(chain, x)
	}
	for i := len(chain) - 1; i >= 0; i-- {
		x := chain[i]
		for j := range x.Ivars {
			iv := &x.Ivars[j]
			if x != k && iv.Vis == types.VisPrivate {
				continue // not visible here
			}
			c.declareName(iv.Name, &symbol{
				kind: symIvar, typ: iv.Type, node: at, ivar: iv,
			})
		}
	}
}

// findProperty resolves a property name against what an object type knows.
func (c *checker) findProperty(o *types.Object, name string) *types.Property {
	if o.Base != nil {
		if p := o.Base.FindProperty(name); p != nil {
			return p
		}
	}
	for _, proto := range o.Protocols {
		if p := proto.FindProperty(name); p != nil {
			return p
		}
	}
	return nil
}
