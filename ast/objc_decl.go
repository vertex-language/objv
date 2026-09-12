package ast

import "github.com/vertex-language/objv/token"

// ClassInterfaceDecl represents an @interface declaration (§4.1).
type ClassInterfaceDecl struct {
	Span
	Attrs      []*Attr
	Keyword    token.Pos
	Name       *Ident
	TypeParams *TypeParamList
	Colon      token.Pos // NoPos for a root class
	Super      *Ident
	SuperArgs  *TypeArgList
	Protocols  *ProtocolRefList
	Ivars      *IvarList
	Members    []Decl
	EndKeyword token.Pos // the @end
}

// ClassImplDecl represents an @implementation declaration (§4.1).
type ClassImplDecl struct {
	Span
	Attrs      []*Attr
	Keyword    token.Pos
	Name       *Ident
	Colon      token.Pos // NoPos when the superclass is not restated
	Super      *Ident
	Ivars      *IvarList
	Members    []Decl
	EndKeyword token.Pos // the @end
}

// CategoryDecl represents a category interface or class extension (§4.2).
// When Name is nil, it is a class extension (@interface Foo ()).
type CategoryDecl struct {
	Span
	Attrs      []*Attr
	Keyword    token.Pos
	Class      *Ident
	TypeParams *TypeParamList
	Lparen     token.Pos
	Name       *Ident // nil for a class extension
	Rparen     token.Pos
	Protocols  *ProtocolRefList
	Ivars      *IvarList
	Members    []Decl
	EndKeyword token.Pos // the @end
}

// IsExtension reports whether the category name was omitted, which makes this
// a class extension rather than a category.
func (d *CategoryDecl) IsExtension() bool { return d.Name == nil }

// CategoryImplDecl is §4.2's CategoryImplementation.
type CategoryImplDecl struct {
	Span
	Keyword    token.Pos
	Class      *Ident
	Lparen     token.Pos
	Name       *Ident
	Rparen     token.Pos
	Members    []Decl
	EndKeyword token.Pos // the @end
}

// ProtocolDecl represents an @protocol declaration (§4.3).
type ProtocolDecl struct {
	Span
	Attrs      []*Attr
	Keyword    token.Pos
	Name       *Ident
	Protocols  *ProtocolRefList
	Members    []Decl
	EndKeyword token.Pos // the @end
}

// RequirementDecl is an @required or @optional marker inside a protocol.
type RequirementDecl struct {
	Span
	Keyword token.Pos
	Kind    token.Kind // AT_REQUIRED or AT_OPTIONAL
}

// ClassForwardDecl is §4.4's `@class A, B<T>;`.
type ClassForwardDecl struct {
	Span
	Keyword token.Pos
	Names   []*ForwardClass
	Semi    token.Pos
}

// ForwardClass is one name of an @class list, with the type parameters a
// generic class may declare there.
type ForwardClass struct {
	Span
	Name       *Ident
	TypeParams *TypeParamList
}

// ProtocolForwardDecl is §4.4's `@protocol A, B;`.
//
// It is told from a ProtocolDecl by what follows the name list: a semicolon
// here, a protocol body there. The parser decides with one token of
// lookahead past the first name.
type ProtocolForwardDecl struct {
	Span
	Keyword token.Pos
	Names   []*Ident
	Semi    token.Pos
}

// CompatAliasDecl is §4.4's `@compatibility_alias Alias Class;`.
type CompatAliasDecl struct {
	Span
	Keyword token.Pos
	Alias   *Ident
	Class   *Ident
	Semi    token.Pos
}

// ImportDecl is §4.4's `@import Foundation.NSString;` — a module import, not
// a preprocessing directive. It carries no '#', survives phase 4 untouched,
// and each name after the first names a submodule of the one before it.
type ImportDecl struct {
	Span
	Keyword token.Pos
	Path    []*Ident
	Semi    token.Pos
}

// IvarList represents brace-enclosed instance variables (§4.5).
type IvarList struct {
	Span
	Lbrace token.Pos
	Items  []Decl
	Rbrace token.Pos
}

// VisibilityDecl is one @private / @protected / @public / @package marker.
type VisibilityDecl struct {
	Span
	Keyword token.Pos
	Kind    token.Kind // AT_PRIVATE, AT_PROTECTED, AT_PUBLIC, AT_PACKAGE
}

// MethodDecl represents an Objective-C method declaration or definition (§4.7).
// Body is nil for declarations and non-nil for definitions.
type MethodDecl struct {
	Span
	Keyword   token.Pos
	Kind      token.Kind // SUB (-) or ADD (+)
	Type      *MethodType
	Attrs     []*Attr
	Sel       *Ident
	Parts     []*KeywordDecl
	Params    []*ParamDecl
	Ellipsis  token.Pos
	TailAttrs []*Attr
	KR        []*GenDecl // parameter declarations after the selector; obsolescent
	Semi      token.Pos  // NoPos in a definition
	Body      *CompoundStmt
}

// IsDefinition reports whether the method has a body.
func (d *MethodDecl) IsDefinition() bool { return d.Body != nil }

// IsClassMethod reports whether the method was written with '+'.
func (d *MethodDecl) IsClassMethod() bool { return d.Kind == token.ADD }

// KeywordDecl is one `[Selector] : [MethodType] [Attrs] Identifier` of a keyword method.
type KeywordDecl struct {
	Span
	Sel   *Ident
	Colon token.Pos
	Type  *MethodType
	Attrs []*Attr
	Name  *Ident
}

// MethodType is `( {ProtocolQualifier} [TypeName] )` for return or parameter types (§4.7).
type MethodType struct {
	Span
	Lparen token.Pos
	Quals  []*ProtoQual
	Type   *TypeName
	Rparen token.Pos
}

// ProtoQual is a distributed-object qualifier (in, out, inout, bycopy, byref, oneway).
type ProtoQual struct {
	Span
	Kind ProtoQualKind
}

// ProtoQualKind names the six qualifiers of §4.7's ProtocolQualifier.
type ProtoQualKind uint8

const (
	QualIn ProtoQualKind = iota
	QualOut
	QualInout
	QualBycopy
	QualByref
	QualOneway
)

func (k ProtoQualKind) String() string {
	switch k {
	case QualIn:
		return "in"
	case QualOut:
		return "out"
	case QualInout:
		return "inout"
	case QualBycopy:
		return "bycopy"
	case QualByref:
		return "byref"
	case QualOneway:
		return "oneway"
	}
	return "ProtoQualKind(?)"
}

// PropertyDecl represents a @property declaration (§4.8).
type PropertyDecl struct {
	Span
	Keyword token.Pos
	Lparen  token.Pos
	Attrs   []*PropertyAttr
	Rparen  token.Pos
	Specs   DeclSpecs
	List    []Declarator
	Semi    token.Pos
}

// PropertyAttr is an entry in a property attribute list (e.g. nonatomic, copy, getter=foo).
type PropertyAttr struct {
	Span
	Kind   PropertyAttrKind
	Name   *Ident // the attribute name as written
	Assign token.Pos
	Sel    *Ident
	Colon  token.Pos
}

// PropertyAttrKind names §4.8's closed set. An identifier in a property
// attribute list that is none of these is a syntax error, not a semantic one.
type PropertyAttrKind uint8

const (
	PropClass PropertyAttrKind = iota
	PropDirect
	PropAtomic
	PropNonatomic
	PropReadonly
	PropReadwrite
	PropAssign
	PropRetain
	PropCopy
	PropStrong
	PropWeak
	PropUnsafeUnretained
	PropNullable
	PropNonnull
	PropNullResettable
	PropNullUnspecified
	PropGetter
	PropSetter
)

func (k PropertyAttrKind) String() string {
	switch k {
	case PropClass:
		return "class"
	case PropDirect:
		return "direct"
	case PropAtomic:
		return "atomic"
	case PropNonatomic:
		return "nonatomic"
	case PropReadonly:
		return "readonly"
	case PropReadwrite:
		return "readwrite"
	case PropAssign:
		return "assign"
	case PropRetain:
		return "retain"
	case PropCopy:
		return "copy"
	case PropStrong:
		return "strong"
	case PropWeak:
		return "weak"
	case PropUnsafeUnretained:
		return "unsafe_unretained"
	case PropNullable:
		return "nullable"
	case PropNonnull:
		return "nonnull"
	case PropNullResettable:
		return "null_resettable"
	case PropNullUnspecified:
		return "null_unspecified"
	case PropGetter:
		return "getter"
	case PropSetter:
		return "setter"
	}
	return "PropertyAttrKind(?)"
}

// PropertyImplDecl is §4.8's @synthesize and @dynamic.
type PropertyImplDecl struct {
	Span
	Keyword token.Pos
	Kind    token.Kind // AT_SYNTHESIZE or AT_DYNAMIC
	Items   []*PropertyImplItem
	Semi    token.Pos
}

// PropertyImplItem is one `Identifier [= Identifier]`: the property, and the
// instance variable backing it.
type PropertyImplItem struct {
	Span
	Name   *Ident
	Assign token.Pos // NoPos when no ivar is named
	Ivar   *Ident
}

func (*ClassInterfaceDecl) declNode()  {}
func (*ClassImplDecl) declNode()       {}
func (*CategoryDecl) declNode()        {}
func (*CategoryImplDecl) declNode()    {}
func (*ProtocolDecl) declNode()        {}
func (*RequirementDecl) declNode()     {}
func (*ClassForwardDecl) declNode()    {}
func (*ProtocolForwardDecl) declNode() {}
func (*CompatAliasDecl) declNode()     {}
func (*ImportDecl) declNode()          {}
func (*VisibilityDecl) declNode()      {}
func (*MethodDecl) declNode()          {}
func (*PropertyDecl) declNode()        {}
func (*PropertyImplDecl) declNode()    {}
