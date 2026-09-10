package ast

import "github.com/vertex-language/objv/token"

// ClassInterfaceDecl is §4.1's @interface: the class's name, what it inherits
// from, what it conforms to, its instance variables, and its members.
//
//	@interface Cache<KeyType> : NSObject <NSCopying> { … } … @end
//
// SuperArgs and Protocols are both angle-bracket lists after the superclass
// and are told apart by resolving each name (§4.1): a protocol name makes a
// ProtocolRefList, anything else a TypeArgList. Both may appear, in that
// order.
//
// Attrs are the attributes written *before* @interface. §5.9 forbids them
// between the keyword and the class name, which is why there is one field and
// not two.
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

// ClassImplDecl is §4.1's @implementation.
//
// It takes neither a TypeParamList nor a TypeArgList: lightweight generics
// are erased and exist only in the interface. Attributes are accepted before
// it and have no effect there, so they are kept for a formatter and ignored
// by everything else.
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

// CategoryDecl is §4.2's CategoryInterface, and its ClassExtension when Name
// is nil.
//
//	@interface Cache (Persistence) <NSCoding> … @end   // a category
//	@interface Cache () { … } … @end                   // a class extension
//
// The two are one node because they are one production with one part omitted,
// and because what differs is what they may contain rather than how they are
// written: an extension may declare instance variables and add to the class's
// own interface, and a category may not. IsExtension says which.
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

// ProtocolDecl is §4.3's @protocol declaration.
//
// Members holds the sections flattened into written order, with a
// RequirementDecl standing where each @required or @optional was written. A
// protocol's members default to required until the first marker, and each
// marker holds until the next — state the analyzer carries as it walks, which
// is also how it reports one written where §4.3 does not allow it.
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

// IvarList is §4.5's brace-enclosed instance variables.
//
// Items are *FieldDecl and *StaticAssertDecl in written order, with a
// *VisibilityDecl standing where each @private, @protected, @public or
// @package was written. Variables before the first marker take a default that
// depends on the enclosing construct — @protected in a class interface,
// @private in an implementation, extension or category — which is why the
// default is not recorded here: this node does not know what encloses it.
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

// MethodDecl is §4.7's method declaration and method definition: the same
// node, with Body nil for a declaration and non-nil for a definition.
//
//   - (void)setObject:(id)obj forKey:(id<NSCopying>)key;
//   - (instancetype)cacheWithCapacity:(NSUInteger)cap;
//
// Kind is SUB for an instance method and ADD for a class method, which is the
// punctuator each is written with.
//
// A unary method fills Sel and leaves Parts empty; a keyword method fills
// Parts and leaves Sel nil. Params and Ellipsis are §4.7's
// MethodParameterSuffix, the C-style trailing parameters that make a method
// variadic.
//
// Attributes appear in three positions on a method and are kept apart because
// they attach to different things: Attrs before the selector, each
// KeywordDecl's own between its type and its parameter name, and TailAttrs
// after the complete selector — the position NS_DESIGNATED_INITIALIZER,
// NS_SWIFT_NAME and the deprecation macros all use.
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

// KeywordDecl is one `[Selector] : [MethodType] [Attrs] Identifier` of a
// keyword method's selector. Sel is nil where the piece has no name, which
// makes `- (void)a::(int)x` a method named a::.
type KeywordDecl struct {
	Span
	Sel   *Ident
	Colon token.Pos
	Type  *MethodType
	Attrs []*Attr
	Name  *Ident
}

// MethodType is §4.7's `( {ProtocolQualifier} [TypeName] )`: the
// parenthesized type of a method's return value or of one keyword's
// parameter.
//
// Type is nil where only distributed-object qualifiers were written —
// `- (oneway)shutdown;` is well-formed and states no type. An empty `()` is
// derivable and is rejected by diagnosis rather than by parse failure, so a
// MethodType with neither quals nor type can exist in a tree the parser has
// already reported on.
type MethodType struct {
	Span
	Lparen token.Pos
	Quals  []*ProtoQual
	Type   *TypeName
	Rparen token.Pos
}

// ProtoQual is one of §4.7's distributed-object qualifiers. They lex as
// identifiers and mean something only inside a MethodType, so the parser
// resolves them by position and stores what it resolved.
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

// PropertyDecl is §4.8's @property.
//
//	@property (nonatomic, copy) NSString *first, *last;
//
// One property is declared per declarator, so that line declares two. A
// property declarator may not carry a bit-field width and may not be
// abstract — constraints, checked where constraints are.
//
// Lparen is NoPos when no attribute list was written; an empty list is not
// the same thing and is legal (`@property () NSString *name;`).
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

// PropertyAttr is one entry of a property attribute list: a name from §4.8's
// closed set, or `getter = Selector`, or `setter = Selector :`.
//
// Sel is the selector named by a getter or setter attribute and is nil
// otherwise. Colon is the trailing colon of a setter's selector, which is
// part of the selector rather than punctuation between attributes.
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
