package lower

import (
	"strings"

	"github.com/vertex-language/ir"
	"github.com/vertex-language/objv/analyzer"
	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/runtime"
	"github.com/vertex-language/objv/token"
	"github.com/vertex-language/objv/types"
)

// §6.8's literals: the syntax that means an object nobody wrote a send for.
//
// All but one of them *are* sends — @42 is a numberWith… to NSNumber, @[…]
// an arrayWithObjects:count: to NSArray — which is why they are here rather
// than in expr.go and why they need Foundation to be linked. The exception is
// @"…", which is a constant object in the image: it has to work before any
// class is realized, so it is data and not a call.

// constantString lowers @"…".
//
// On Darwin the object is a CFString, whose isa is a symbol CoreFoundation
// defines. Its characters live beside it, 8-bit where they fit in ASCII and
// UTF-16 where they do not, and the length field counts code units rather
// than bytes — so a literal with one non-ASCII character changes encoding,
// section, and count all at once.
func (u *unit) constantString(e *ast.StringLit) ir.Value {
	sym := u.constantStringSymbol(e)
	if sym == nil {
		return nil
	}
	return u.fn.cur.Ptr.GetAddr(sym)
}

// constantStringSymbol is the object itself, for the two callers that want
// different things from it: an expression wants its address, and a file-scope
// initializer wants a relocation naming it. A @"…" is the one Objective-C
// object a program may name before any class is realized, which is what makes
// the second possible at all — `NSString *const D = @"…";` is every
// framework's error domain.
func (u *unit) constantStringSymbol(e *ast.StringLit) ir.Symbol {
	val := analyzer.DecodeString(u.src, e, u.model, func(string) {})
	units := val.Data
	if n := len(units); n > 0 && units[n-1] == 0 {
		units = units[:n-1] // the terminator is not part of the length
	}

	ascii := true
	for _, c := range units {
		if c > 0x7F {
			ascii = false
			break
		}
	}

	key := "cfstr\x00"
	var data ir.Symbol
	flags := runtime.CFStringASCII
	if ascii {
		var sb strings.Builder
		for _, c := range units {
			sb.WriteByte(byte(c))
		}
		key += sb.String()
		if s, ok := u.strs[key]; ok {
			return s
		}
		data = u.cstringIn(sb.String(), runtime.SecCString, runtime.CStringLabel)
	} else {
		// UTF-16, which the decoder did not produce: DecodeString yields
		// UTF-8 bytes for a plain literal, and what CoreFoundation wants
		// is code units. Re-decoding here rather than asking for wide
		// output keeps one decoder in the compiler.
		w := utf16Of(units)
		var sb strings.Builder
		for _, c := range w {
			sb.WriteByte(byte(c))
			sb.WriteByte(byte(c >> 8))
		}
		key += sb.String()
		if s, ok := u.strs[key]; ok {
			return s
		}
		units = w
		flags = runtime.CFStringUTF16
		items := make([]ir.Init, 0, len(w)+1)
		for _, c := range w {
			items = append(items, ir.Lit(ir.Int(int64(c))))
		}
		items = append(items, ir.Lit(ir.Int(0)))
		data = u.mod.Global(u.sym(u.uniq(runtime.CStringLabel)), ir.RO,
			ir.Array(uint64(len(items)), ir.StoreI16.FType())).
			Internal().
			Section(u.abi.Name(runtime.SecUString)).
			Align(2).
			Init(ir.List(items...))
	}

	isa := u.classSymbol(u.abi.ConstantStringClass())
	g := u.mod.Global(u.sym(u.uniq(runtime.ConstStringLabel)), ir.RW,
		u.metaType("objc_constant_string", runtime.ConstantString).FType()).
		Internal().
		Section(u.abi.Name(runtime.SecCFString)).
		Align(uint64(u.abi.PtrBytes)).
		Init(ir.List(
			ir.RelocInit(isa),
			ir.Lit(ir.Int(int64(flags))),
			ir.Lit(ir.Int(0)), // the four bytes after flags on a 64-bit target
			ir.RelocInit(data),
			ir.Lit(ir.Int(int64(len(units))))))
	u.strs[key] = g
	return g
}

// utf16Of re-reads a decoded literal's UTF-8 bytes as UTF-16 code units. A
// code point outside the BMP becomes a surrogate pair, which is why the
// result is not one unit per rune.
func utf16Of(bytes []uint32) []uint32 {
	b := make([]byte, len(bytes))
	for i, c := range bytes {
		b[i] = byte(c)
	}
	var out []uint32
	for _, r := range string(b) {
		if r > 0xFFFF {
			r -= 0x10000
			out = append(out, uint32(0xD800+(r>>10)), uint32(0xDC00+(r&0x3FF)))
			continue
		}
		out = append(out, uint32(r))
	}
	return out
}

// boxed lowers @42, @'c', @YES and @(expr).
//
// Every form is a class message to NSNumber or NSString; which one is
// decided by the type of what was boxed, exactly as §6.8 describes it. A
// type with no boxing method is an error the analyzer already reported, and
// what reaches here is only the shapes it admitted.
func (u *unit) boxed(e *ast.BoxedExpr, t types.Type) ir.Value {
	xt := u.typeOf(e.X)
	class, sel := boxingMethod(xt)

	// @__objc_yes is a boolean whatever BOOL was typedef'd as. The
	// literal says so and the type does not: BOOL is signed char on
	// Darwin, and boxing one as a character would give back an NSNumber
	// that prints as 1 rather than as YES.
	if lit, ok := stripParens(e.X).(*ast.BasicLit); ok && lit.Kind == token.BOOL_LIT {
		class, sel = "NSNumber", "numberWithBool:"
	}
	// A struct marked objc_boxable boxes as an NSValue of its bytes and
	// its encoding, which is what clang sends: the value is held by
	// address here, so the address is already the bytes' pointer.
	if class == "" && types.IsRecord(xt) {
		v, ok := u.rvalue(e.X).(ir.Ptr)
		if !ok {
			return nil
		}
		enc := u.cstring(u.abi.Encode(xt, u.model))
		return u.sendToClass("NSValue", "valueWithBytes:objCType:", []ir.Value{v, enc}, t, e)
	}
	if class == "" {
		u.unsupported(e, "boxing a value of type "+xt.String())
		return nil
	}
	v := u.rvalue(e.X)
	if v == nil {
		return nil
	}
	// A C string boxes through stringWithUTF8String:, which takes the
	// pointer as it stands; everything else boxes the value itself.
	return u.sendToClass(class, sel, []ir.Value{v}, t, e)
}

// boxingMethod is §6.8's table: the class and selector a type boxes through.
func boxingMethod(t types.Type) (string, string) {
	if types.IsBool(t) {
		return "NSNumber", "numberWithBool:"
	}
	// A C string, as a pointer or as the array a literal or a buffer is:
	// the array decays to the pointer stringWithUTF8String: takes.
	var elem types.Type
	if p := types.AsPointer(types.Unqualify(t)); p != nil {
		elem = p.Elem
	} else if a := types.AsArray(types.Unqualify(t)); a != nil {
		elem = a.Elem
	}
	if elem != nil {
		switch types.Unqualify(elem).Kind() {
		case types.Char, types.SChar, types.UChar:
			return "NSString", "stringWithUTF8String:"
		}
		return "", ""
	}
	// An enumeration boxes as the integer it is, which is what §6.8 means
	// by "the enumerator's type" — NSNumber has no enum method.
	k := types.Unqualify(t).Kind()
	if e, ok := types.Unqualify(t).(*types.Enum); ok {
		k = e.Underlying()
	}
	name := map[types.Kind]string{
		types.Char:      "numberWithChar:",
		types.SChar:     "numberWithChar:",
		types.UChar:     "numberWithUnsignedChar:",
		types.Short:     "numberWithShort:",
		types.UShort:    "numberWithUnsignedShort:",
		types.Int:       "numberWithInt:",
		types.UInt:      "numberWithUnsignedInt:",
		types.Long:      "numberWithLong:",
		types.ULong:     "numberWithUnsignedLong:",
		types.LongLong:  "numberWithLongLong:",
		types.ULongLong: "numberWithUnsignedLongLong:",
		types.Float:     "numberWithFloat:",
		types.Double:    "numberWithDouble:",
	}[k]
	if name == "" {
		return "", ""
	}
	return "NSNumber", name
}

// arrayLit lowers @[a, b, c].
//
// The elements are written into a frame array and the address handed to
// arrayWithObjects:count:, because that is the only signature the method
// has: there is no varargs form the compiler could use, and the array has to
// outlive the argument setup, which is why it is a slot and not a temporary.
func (u *unit) arrayLit(e *ast.ArrayLit, t types.Type) ir.Value {
	vals, ok := u.objectValues(e.Elems)
	if !ok {
		return nil
	}
	buf := u.pointerArray(vals, "objects")
	n := u.fn.cur.I64.Const(int64(len(vals)))
	return u.sendToClass("NSArray", "arrayWithObjects:count:",
		[]ir.Value{buf, n}, t, e)
}

// dictLit lowers @{k: v, …}.
//
// The two arrays are parallel and the keys array is second, which is the
// order the selector states and not the order the syntax writes: a
// dictionary literal is read key-first and passed value-first.
func (u *unit) dictLit(e *ast.DictLit, t types.Type) ir.Value {
	keys := make([]ast.Expr, 0, len(e.Pairs))
	vals := make([]ast.Expr, 0, len(e.Pairs))
	for _, p := range e.Pairs {
		keys = append(keys, p.Key)
		vals = append(vals, p.Value)
	}
	// Left to right across the whole literal, which is the order the
	// elements are written in and the order their side effects happen.
	var objs, ks []ir.Value
	for i := range e.Pairs {
		k := u.rvalue(keys[i])
		v := u.rvalue(vals[i])
		if k == nil || v == nil {
			return nil
		}
		ks = append(ks, k)
		objs = append(objs, v)
	}
	ob := u.pointerArray(objs, "objects")
	kb := u.pointerArray(ks, "keys")
	n := u.fn.cur.I64.Const(int64(len(e.Pairs)))
	return u.sendToClass("NSDictionary", "dictionaryWithObjects:forKeys:count:",
		[]ir.Value{ob, kb, n}, t, e)
}

// objectValues lowers a literal's elements in order.
func (u *unit) objectValues(es []ast.Expr) ([]ir.Value, bool) {
	out := make([]ir.Value, 0, len(es))
	for _, x := range es {
		v := u.rvalue(x)
		if v == nil {
			return nil, false
		}
		out = append(out, v)
	}
	return out, true
}

// pointerArray writes values into a frame array and returns its address.
func (u *unit) pointerArray(vals []ir.Value, name string) ir.Ptr {
	b := u.fn.cur
	n := int64(len(vals))
	if n == 0 {
		n = 1 // an empty literal still passes an address
	}
	buf := u.fn.entry.Ptr.Alloc(uint64(n*u.abi.PtrBytes), uint64(u.abi.PtrBytes))
	u.fn.entry.Name(buf, name)
	for i, v := range vals {
		p, ok := v.(ir.Ptr)
		if !ok {
			continue
		}
		b.Ptr.Store(p, b.Ptr.Add(buf, b.I64.Const(int64(i)*u.abi.PtrBytes)))
	}
	return buf
}
