package ast

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/vertex-language/objv/token"
)

// The hierarchy lists below are the inventory, and they are a compile-time
// check as well as data: a node whose marker method is missing — or whose
// field named End or Pos silently shadows the method Span provides — does not
// go in the slice, and the package stops building.
//
// allNodes is the same inventory including the nodes that belong to no
// hierarchy, and TestInventoryIsComplete keeps the two in step.

// tree hand-builds the AST for a one-line method declaration over a real
// token.File.
//
//   - (void)greet:(NSString *)name;
//     0    5    10   15   20   25   30
func tree(t *testing.T) (*token.File, *MethodDecl) {
	t.Helper()
	const src = "- (void)greet:(NSString *)name;\n"
	f := token.NewFile("a.m", []byte(src))
	sp := func(lo, hi int) Span { return Span{f.Pos(lo), f.Pos(hi)} }
	return f, &MethodDecl{
		Span:    sp(0, 31),
		Keyword: f.Pos(0),
		Kind:    token.SUB,
		Type: &MethodType{
			Span:   sp(2, 8),
			Lparen: f.Pos(2),
			Type: &TypeName{
				Span:  sp(3, 7),
				Specs: DeclSpecs{&KeywordSpec{sp(3, 7), token.VOID}},
			},
			Rparen: f.Pos(7),
		},
		Parts: []*KeywordDecl{{
			Span:  sp(8, 30),
			Sel:   &Ident{sp(8, 13)},
			Colon: f.Pos(13),
			Type: &MethodType{
				Span:   sp(14, 26),
				Lparen: f.Pos(14),
				Type: &TypeName{
					Span: sp(15, 25),
					Specs: DeclSpecs{&ObjectType{
						Span: sp(15, 23), Kind: ObjectNamed, Name: &Ident{sp(15, 23)},
					}},
					Decl: &PtrDeclarator{Span: sp(24, 25), Star: f.Pos(24)},
				},
				Rparen: f.Pos(25),
			},
			Name: &Ident{sp(26, 30)},
		}},
		Semi: f.Pos(30),
	}
}

func TestWalkOrder(t *testing.T) {
	_, d := tree(t)
	var got []string
	Inspect(d, func(n Node) bool {
		got = append(got, nodeName(n))
		return true
	})
	want := []string{
		"MethodDecl", "MethodType", "TypeName", "KeywordSpec",
		"KeywordDecl", "Ident", "MethodType", "TypeName", "ObjectType", "Ident",
		"PtrDeclarator", "Ident",
	}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("walk order =\n %v\nwant\n %v", got, want)
	}
}

func TestInspectSkipsSubtree(t *testing.T) {
	_, d := tree(t)
	var got []string
	Inspect(d, func(n Node) bool {
		got = append(got, nodeName(n))
		return nodeName(n) != "KeywordDecl"
	})
	for _, name := range got[1:] {
		if name == "PtrDeclarator" {
			t.Fatalf("subtree not skipped: %v", got)
		}
	}
}

func TestTypedNilsSkipped(t *testing.T) {
	_, d := tree(t)
	s := &TryStmt{
		Span:    d.Span,
		Body:    &CompoundStmt{Span: d.Span},
		Finally: nil,
		Catches: []*CatchClause{{Span: d.Span, Body: (*CompoundStmt)(nil)}},
	}
	var got []string
	Inspect(s, func(n Node) bool {
		got = append(got, nodeName(n))
		return true
	})
	if want := "TryStmt CompoundStmt CatchClause"; strings.Join(got, " ") != want {
		t.Fatalf("got %v, want %q", got, want)
	}
}

// An alias field must not be walked twice: FuncDecl.Name is the same *Ident
// the declarator holds.
func TestAliasFieldsNotWalkedTwice(t *testing.T) {
	f := token.NewFile("a.m", []byte("int main(void) {}\n"))
	sp := func(lo, hi int) Span { return Span{f.Pos(lo), f.Pos(hi)} }
	name := &Ident{sp(4, 8)}
	fn := &FuncDecl{
		Span:  sp(0, 17),
		Specs: DeclSpecs{&KeywordSpec{sp(0, 3), token.INT}},
		Decl: &FuncDeclarator{
			Span:   sp(4, 14),
			Inner:  &NameDeclarator{sp(4, 8), name},
			Lparen: f.Pos(8),
			Rparen: f.Pos(13),
		},
		Name: name,
		Body: &CompoundStmt{Span: sp(15, 17), Lbrace: f.Pos(15), Rbrace: f.Pos(16)},
	}
	n := 0
	Inspect(fn, func(node Node) bool {
		if node == Node(name) {
			n++
		}
		return true
	})
	if n != 1 {
		t.Errorf("the name was walked %d times, want 1", n)
	}
}

// Tokens holds a BalancedTokenSequence, which is data and not children.
func TestTokensAreNotChildren(t *testing.T) {
	f := token.NewFile("a.m", []byte(`asm("nop");`+"\n"))
	sp := func(lo, hi int) Span { return Span{f.Pos(lo), f.Pos(hi)} }
	s := &AsmStmt{
		Span:    sp(0, 11),
		Keyword: f.Pos(0),
		Kind:    token.ASM,
		Lparen:  f.Pos(3),
		Body: &Tokens{
			Span: sp(4, 9),
			List: []token.Token{{Kind: token.STRING_LIT, Pos: f.Pos(4), End: f.Pos(9)}},
		},
		Rparen: f.Pos(9),
		Semi:   f.Pos(10),
	}
	var got []string
	Inspect(s, func(n Node) bool {
		got = append(got, nodeName(n))
		return true
	})
	if want := "AsmStmt Tokens"; strings.Join(got, " ") != want {
		t.Fatalf("got %v, want %q", got, want)
	}
}

// The same node stands in two hierarchies, because §3 lists an AsmStatement
// among the external declarations.
func TestAsmIsStatementAndDeclaration(t *testing.T) {
	var s Stmt = (*AsmStmt)(nil)
	var d Decl = (*AsmStmt)(nil)
	if reflect.TypeOf(s) != reflect.TypeOf(d) {
		t.Fatal("AsmStmt must be one type in both hierarchies")
	}
}

func TestDeclName(t *testing.T) {
	f := token.NewFile("a.m", []byte("void (^b)(int);\n"))
	sp := func(lo, hi int) Span { return Span{f.Pos(lo), f.Pos(hi)} }
	name := &Ident{sp(7, 8)}
	// void (^b)(int) — the name is inside a block pointer inside parentheses.
	d := &FuncDeclarator{
		Span: sp(5, 14),
		Inner: &ParenDeclarator{
			Span:   sp(5, 9),
			Lparen: f.Pos(5),
			Inner: &BlockPtrDeclarator{
				Span:  sp(6, 8),
				Caret: f.Pos(6),
				Inner: &NameDeclarator{sp(7, 8), name},
			},
			Rparen: f.Pos(8),
		},
		Lparen: f.Pos(9),
		Rparen: f.Pos(13),
	}
	if got := d.DeclName(); got != name {
		t.Errorf("DeclName() = %v, want the identifier inside the block pointer", got)
	}
	if got := (&PtrDeclarator{}).DeclName(); got != nil {
		t.Errorf("an abstract declarator has no name, got %v", got)
	}
}

func TestSpansAreStored(t *testing.T) {
	f, d := tree(t)
	if got := string(f.Slice(d.Pos(), d.End())); got != "- (void)greet:(NSString *)name;" {
		t.Errorf("span covers %q", got)
	}
	Inspect(d, func(n Node) bool {
		if !n.Pos().IsValid() || n.End() <= n.Pos() {
			t.Errorf("%s has an empty span [%d,%d)", nodeName(n), n.Pos(), n.End())
		}
		return true
	})
}

func TestFdump(t *testing.T) {
	f, d := tree(t)
	var b bytes.Buffer
	if err := Fdump(&b, f, d); err != nil {
		t.Fatal(err)
	}
	got := b.String()
	for _, want := range []string{
		"MethodDecl 1:1 -",     // the punctuator that says instance method
		"KeywordSpec 1:4 void", // resolved through the File
		"Ident 1:9 greet",
		"ObjectType 1:16 class",
		"Ident 1:16 NSString", // the span must cover the name and nothing else
		"Ident 1:27 name",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("dump is missing %q:\n%s", want, got)
		}
	}
}

// Every node embeds Span, and no node has a field that shadows the two
// methods Span provides. The inventory above is the compile-time half of this
// check; this is the half that names what went wrong.
func TestEveryNodeEmbedsSpan(t *testing.T) {
	for _, n := range allNodes() {
		rt := reflect.TypeOf(n).Elem()
		f, ok := rt.FieldByName("Span")
		if !ok || !f.Anonymous {
			t.Errorf("%s does not embed Span", rt.Name())
			continue
		}
		for _, bad := range []string{"Pos", "End"} {
			if fld, ok := rt.FieldByName(bad); ok && len(fld.Index) == 1 {
				t.Errorf("%s.%s shadows the method Span provides; rename the field",
					rt.Name(), bad)
			}
		}
	}
}

// allNodes returns one zero value of every node type in the package.
func allNodes() []Node {
	var out []Node
	add := func(ns ...Node) { out = append(out, ns...) }
	add((*File)(nil), (*Tokens)(nil), (*Ident)(nil))
	add((*BadExpr)(nil), (*BasicLit)(nil), (*StringLit)(nil), (*ParenExpr)(nil),
		(*GenericExpr)(nil), (*GenericAssoc)(nil), (*IndexExpr)(nil), (*CallExpr)(nil),
		(*MemberExpr)(nil), (*IncDecExpr)(nil), (*CompoundLit)(nil), (*UnaryExpr)(nil),
		(*SizeofExpr)(nil), (*AlignofExpr)(nil), (*CastExpr)(nil), (*BinaryExpr)(nil),
		(*CondExpr)(nil), (*AssignExpr)(nil), (*StmtExpr)(nil), (*InitList)(nil),
		(*InitItem)(nil), (*IndexDesignator)(nil), (*FieldDesignator)(nil))
	add((*MessageExpr)(nil), (*KeywordArg)(nil), (*SuperExpr)(nil), (*ClassExpr)(nil),
		(*SelectorExpr)(nil), (*SelectorPart)(nil), (*ProtocolExpr)(nil),
		(*EncodeExpr)(nil), (*BoxedExpr)(nil), (*ArrayLit)(nil), (*DictLit)(nil),
		(*KeyValue)(nil), (*BlockLit)(nil), (*AvailabilityExpr)(nil),
		(*AvailabilitySpec)(nil))
	add((*KeywordSpec)(nil), (*AlignasSpec)(nil), (*TypeofType)(nil), (*AtomicType)(nil),
		(*PtrauthSpec)(nil), (*ObjectType)(nil), (*ProtocolRefList)(nil),
		(*TypeArgList)(nil), (*TypeParamList)(nil), (*TypeParam)(nil), (*StructType)(nil),
		(*DefsSpec)(nil), (*Attr)(nil), (*AttrSpec)(nil), (*EnumDecl)(nil),
		(*Enumerator)(nil), (*TypedefType)(nil), (*TypeName)(nil), (*BadDecl)(nil),
		(*GenDecl)(nil), (*InitDeclarator)(nil), (*FuncDecl)(nil), (*FieldDecl)(nil),
		(*FieldDeclarator)(nil), (*StaticAssertDecl)(nil), (*EmptyDecl)(nil))
	add((*ClassInterfaceDecl)(nil), (*ClassImplDecl)(nil), (*CategoryDecl)(nil),
		(*CategoryImplDecl)(nil), (*ProtocolDecl)(nil), (*RequirementDecl)(nil),
		(*ClassForwardDecl)(nil), (*ForwardClass)(nil), (*ProtocolForwardDecl)(nil),
		(*CompatAliasDecl)(nil), (*ImportDecl)(nil), (*IvarList)(nil),
		(*VisibilityDecl)(nil), (*MethodDecl)(nil), (*KeywordDecl)(nil),
		(*MethodType)(nil), (*ProtoQual)(nil), (*PropertyDecl)(nil),
		(*PropertyAttr)(nil), (*PropertyImplDecl)(nil), (*PropertyImplItem)(nil))
	add((*BadDeclarator)(nil), (*NameDeclarator)(nil), (*PtrDeclarator)(nil),
		(*BlockPtrDeclarator)(nil), (*ParenDeclarator)(nil), (*ArrayDeclarator)(nil),
		(*FuncDeclarator)(nil), (*ParamDecl)(nil))
	add((*BadStmt)(nil), (*LabeledStmt)(nil), (*CaseStmt)(nil), (*CompoundStmt)(nil),
		(*DeclStmt)(nil), (*ExprStmt)(nil), (*EmptyStmt)(nil), (*IfStmt)(nil),
		(*SwitchStmt)(nil), (*WhileStmt)(nil), (*DoStmt)(nil), (*ForStmt)(nil),
		(*GotoStmt)(nil), (*ContinueStmt)(nil), (*BreakStmt)(nil), (*ReturnStmt)(nil),
		(*AsmStmt)(nil))
	add((*ForInStmt)(nil), (*TryStmt)(nil), (*CatchClause)(nil), (*FinallyClause)(nil),
		(*ThrowStmt)(nil), (*SyncStmt)(nil), (*AutoreleasePoolStmt)(nil))
	return out
}

// The package declares no node type that the inventory forgot. Reflection
// cannot enumerate a package's types, so this compares the inventory against
// the marker-method lists above, which the compiler checks.
func TestInventoryIsComplete(t *testing.T) {
	seen := map[string]bool{}
	for _, n := range allNodes() {
		seen[reflect.TypeOf(n).Elem().Name()] = true
	}
	inHierarchy := append(asNodes(exprs()), asNodes(stmts())...)
	inHierarchy = append(inHierarchy, asNodes(decls())...)
	inHierarchy = append(inHierarchy, asNodes(declarators())...)
	for _, n := range inHierarchy {
		if name := reflect.TypeOf(n).Elem().Name(); !seen[name] {
			t.Errorf("%s is in a hierarchy but not in allNodes", name)
		}
	}
}

func asNodes[T Node](xs []T) []Node {
	out := make([]Node, 0, len(xs))
	for _, x := range xs {
		out = append(out, x)
	}
	return out
}

func exprs() []Expr {
	return []Expr{(*Ident)(nil), (*BadExpr)(nil), (*BasicLit)(nil), (*StringLit)(nil),
		(*ParenExpr)(nil), (*GenericExpr)(nil), (*IndexExpr)(nil), (*CallExpr)(nil),
		(*MemberExpr)(nil), (*IncDecExpr)(nil), (*CompoundLit)(nil), (*UnaryExpr)(nil),
		(*SizeofExpr)(nil), (*AlignofExpr)(nil), (*CastExpr)(nil), (*BinaryExpr)(nil),
		(*CondExpr)(nil), (*AssignExpr)(nil), (*StmtExpr)(nil), (*InitList)(nil),
		(*KeywordSpec)(nil), (*AttrSpec)(nil), (*AlignasSpec)(nil), (*AtomicType)(nil),
		(*TypeofType)(nil), (*PtrauthSpec)(nil), (*ObjectType)(nil), (*StructType)(nil),
		(*EnumDecl)(nil), (*TypedefType)(nil), (*TypeName)(nil), (*ProtocolRefList)(nil),
		(*TypeArgList)(nil), (*MessageExpr)(nil), (*SuperExpr)(nil), (*ClassExpr)(nil),
		(*SelectorExpr)(nil), (*ProtocolExpr)(nil), (*EncodeExpr)(nil), (*BoxedExpr)(nil),
		(*ArrayLit)(nil), (*DictLit)(nil), (*BlockLit)(nil), (*AvailabilityExpr)(nil)}
}

func stmts() []Stmt {
	return []Stmt{(*BadStmt)(nil), (*LabeledStmt)(nil), (*CaseStmt)(nil),
		(*CompoundStmt)(nil), (*DeclStmt)(nil), (*ExprStmt)(nil), (*EmptyStmt)(nil),
		(*IfStmt)(nil), (*SwitchStmt)(nil), (*WhileStmt)(nil), (*DoStmt)(nil),
		(*ForStmt)(nil), (*GotoStmt)(nil), (*ContinueStmt)(nil), (*BreakStmt)(nil),
		(*ReturnStmt)(nil), (*AsmStmt)(nil), (*ForInStmt)(nil), (*TryStmt)(nil),
		(*ThrowStmt)(nil), (*SyncStmt)(nil), (*AutoreleasePoolStmt)(nil)}
}

func decls() []Decl {
	return []Decl{(*BadDecl)(nil), (*GenDecl)(nil), (*FuncDecl)(nil), (*FieldDecl)(nil),
		(*StaticAssertDecl)(nil), (*EmptyDecl)(nil), (*ClassInterfaceDecl)(nil),
		(*ClassImplDecl)(nil), (*CategoryDecl)(nil), (*CategoryImplDecl)(nil),
		(*ProtocolDecl)(nil), (*RequirementDecl)(nil), (*ClassForwardDecl)(nil),
		(*ProtocolForwardDecl)(nil), (*CompatAliasDecl)(nil), (*ImportDecl)(nil),
		(*VisibilityDecl)(nil), (*MethodDecl)(nil), (*PropertyDecl)(nil),
		(*PropertyImplDecl)(nil)}
}

func declarators() []Declarator {
	return []Declarator{(*BadDeclarator)(nil), (*NameDeclarator)(nil),
		(*PtrDeclarator)(nil), (*BlockPtrDeclarator)(nil), (*ParenDeclarator)(nil),
		(*ArrayDeclarator)(nil), (*FuncDeclarator)(nil)}
}

// The three enums this package declares are printed in dumps and in
// diagnostics, so a value with no name is a bug that only shows up in
// somebody's error message.
func TestEnumNames(t *testing.T) {
	for k := ObjectID; k <= ObjectTypeParam; k++ {
		if s := k.String(); s == "" || strings.Contains(s, "?") {
			t.Errorf("ObjectKind(%d).String() = %q", k, s)
		}
	}
	for k := QualIn; k <= QualOneway; k++ {
		if s := k.String(); s == "" || strings.Contains(s, "?") {
			t.Errorf("ProtoQualKind(%d).String() = %q", k, s)
		}
	}
	for k := PropClass; k <= PropSetter; k++ {
		if s := k.String(); s == "" || strings.Contains(s, "?") {
			t.Errorf("PropertyAttrKind(%d).String() = %q", k, s)
		}
	}
	// And a value past the end says so rather than lying.
	if s := ObjectKind(200).String(); !strings.Contains(s, "?") {
		t.Errorf("ObjectKind(200).String() = %q", s)
	}
	// The names are the spellings the grammar uses.
	if got := PropUnsafeUnretained.String(); got != "unsafe_unretained" {
		t.Errorf("PropUnsafeUnretained = %q", got)
	}
	if got := QualOneway.String(); got != "oneway" {
		t.Errorf("QualOneway = %q", got)
	}
}
