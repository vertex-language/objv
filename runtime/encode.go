package runtime

import (
	"strconv"
	"strings"

	"github.com/vertex-language/objv/types"
)

// Type encodings: the strings §6.4's @encode yields, and the two longer
// forms the metadata carries.
//
// There are three, and they differ in what they are for:
//
//	Encode           what @encode gives a program: the shape of a value
//	EncodeExtended   the same, with class names kept — for ivars and
//	                 properties, where the runtime wants to know what class
//	                 an object field holds
//	MethodTypes      a method's signature with its frame layout, which is
//	                 what NSInvocation and the forwarding machinery read
//
// The alphabet is objc4's, and it is not the same as any other encoding in
// the compiler. `char *` is `*` and not `^c`; a const pointer is `r` before
// the pointer rather than inside it; a block is `@?` and a function pointer
// is `^?`. Each of those looks like an inconsistency and each is what the
// runtime parses, so each is written here as a case rather than derived.

// Encode returns the @encode string for a type.
func (a ABI) Encode(t types.Type, m types.Model) string {
	var b strings.Builder
	e := &encoder{abi: a, model: m}
	e.encode(&b, t, false)
	return b.String()
}

// EncodeExtended returns the encoding the metadata carries for an instance
// variable or a property: the same string, with the class an object field
// holds written into it.
//
//	@encode(NSString *)     →  @
//	extended                →  @"NSString"
//
// The runtime uses the difference for weak references and for the
// introspection a debugger does. A qualified id keeps its protocols instead:
// `id<NSCopying>` is `@"<NSCopying>"`.
func (a ABI) EncodeExtended(t types.Type, m types.Model) string {
	var b strings.Builder
	e := &encoder{abi: a, model: m, extended: true}
	e.encode(&b, t, false)
	return b.String()
}

// MethodTypes returns a method's type string: the return type, the size of
// the argument frame, and every argument with its offset in that frame.
//
//   - (int)feed:(int)n      →  i20@0:8i16
//   - (void)setName:(id)n   →  v24@0:8@16
//
// The two hidden arguments are always there and always first — self at 0 and
// _cmd at the width of a pointer — which is what makes every method callable
// through one function pointer type.
//
// The frame size is the sum of the aligned argument sizes and nothing else:
// it is not rounded up at the end, and a struct returned indirectly does not
// appear in it. Both facts are visible in what clang emits and neither is
// derivable from the calling convention, so both are stated here.
func (a ABI) MethodTypes(ret types.Type, params []types.Param, m types.Model) string {
	// self, then _cmd.
	return a.signature(ret, []types.Type{types.ID(), types.NewSelector()}, params, m)
}

// BlockTypes is the same string for a block's invoke function, which the
// descriptor carries when BlockHasSignature is set.
//
// A block has one hidden argument where a method has two, and it is the
// block itself: `int (^)(int)` encodes as "i12@?0i8" — an int returned, a
// twelve-byte frame, the block at 0 and the int at 8. The runtime reads it
// to build an NSMethodSignature, which is how -[NSInvocation invoke] can
// call a block it was handed.
func (a ABI) BlockTypes(ret types.Type, params []types.Param, m types.Model) string {
	return a.signature(ret, []types.Type{&types.Block{}}, params, m)
}

// signature is the shared shape: the return type, the frame size, and then
// every argument with the offset it sits at.
func (a ABI) signature(ret types.Type, hidden []types.Type, params []types.Param, m types.Model) string {
	var b strings.Builder
	e := &encoder{abi: a, model: m}

	e.encode(&b, ret, false)
	frame := &strings.Builder{}
	off := int64(0)

	for _, h := range hidden {
		frame.WriteString(a.encodeArg(h, m, &off))
	}
	for _, p := range params {
		frame.WriteString(a.encodeArg(p.Type, m, &off))
	}

	b.WriteString(strconv.FormatInt(off, 10))
	b.WriteString(frame.String())
	return b.String()
}

// encodeArg encodes one argument and advances the frame offset past it.
func (a ABI) encodeArg(t types.Type, m types.Model, off *int64) string {
	var b strings.Builder
	e := &encoder{abi: a, model: m}
	e.encode(&b, t, false)
	b.WriteString(strconv.FormatInt(*off, 10))

	size, ok := m.Sizeof(t)
	if !ok || size == 0 {
		size = a.ptr()
	}
	align, ok := m.Alignof(t)
	if !ok || align == 0 {
		align = a.ptr()
	}
	// An argument sits at the next offset its alignment admits, and the
	// frame grows by its size.
	*off = roundUp(*off, align) + size
	return b.String()
}

func roundUp(n, to int64) int64 {
	if to <= 1 {
		return n
	}
	return (n + to - 1) / to * to
}

type encoder struct {
	abi      ABI
	model    types.Model
	extended bool

	// inProgress is the structs and unions whose bodies are being written.
	// A pointer to one of them is encoded as the tag alone: `struct Node {
	// struct Node *next; }` is {Node=^{Node}}, and without this it is not
	// anything, because the expansion does not stop.
	inProgress []*types.Record
}

func (e *encoder) encode(b *strings.Builder, t types.Type, inPointer bool) {
	if t == nil {
		b.WriteByte('?')
		return
	}
	// A const somewhere in a type reaches the encoding as an 'r' in front of
	// whatever it qualifies — but only through a pointer. A top-level const
	// is dropped: `const int` is `i`, and `const char *` is `r*`.
	if inPointer && types.QualsOf(t)&types.QConst != 0 {
		b.WriteByte('r')
	}
	u := types.Unqualify(t)

	if bt, ok := u.(*types.Basic); ok {
		b.WriteString(basicCode(bt.K))
		return
	}
	switch u := u.(type) {
	case *types.Enum:
		// An enumeration encodes as the integer it is compatible with.
		b.WriteString(basicCode(u.Underlying()))

	case *types.TypeParam:
		// A generic parameter is erased to an object pointer.
		b.WriteByte('@')

	case *types.Block:
		b.WriteString("@?")

	case *types.Pointer:
		e.encodePointer(b, u)

	case *types.Array:
		n := u.Len
		if u.Form != types.FixedArray {
			n = 0
		}
		b.WriteByte('[')
		b.WriteString(strconv.FormatInt(n, 10))
		e.encode(b, u.Elem, false)
		b.WriteByte(']')

	case *types.Record:
		e.encodeRecord(b, u, true)

	case *types.Func:
		// A function is not a value, and a pointer to one is '^?'. Reaching
		// here means the type was written where no value can be, which the
		// analyzer has already reported.
		b.WriteByte('?')

	case *types.Object:
		// A bare interface type, which only appears behind a pointer; the
		// pointer case is what writes '@'.
		e.encodeObject(b, u)

	default:
		b.WriteByte('?')
	}
}

// encodePointer writes a pointer, with the three shapes that are not `^`
// followed by the pointee.
func (e *encoder) encodePointer(b *strings.Builder, p *types.Pointer) {
	elem := types.Unqualify(p.Elem)

	// An object pointer is '@', and the interface type inside it is what
	// says which class.
	if o, ok := elem.(*types.Object); ok {
		e.encodeObject(b, o)
		return
	}
	// SEL has a letter of its own. It is structurally a pointer to an
	// incomplete `struct objc_selector`, which is what <objc/objc.h> says it
	// is, and nothing about that shape distinguishes it — the tag does.
	if types.IsSelector(p) {
		b.WriteByte(':')
		return
	}
	// A pointer to a character is '*', whatever the character's signedness
	// spelling was. It is the one type the runtime treats as a string.
	if bt, ok := elem.(*types.Basic); ok && bt.K == types.Char {
		if types.QualsOf(p.Elem)&types.QConst != 0 {
			b.WriteByte('r')
		}
		b.WriteByte('*')
		return
	}
	// A pointer to a function is '^?': the runtime has no encoding for a
	// signature it cannot call blindly.
	if _, ok := elem.(*types.Func); ok {
		b.WriteString("^?")
		return
	}
	if types.QualsOf(p.Elem)&types.QConst != 0 {
		b.WriteByte('r')
	}
	b.WriteByte('^')
	// A pointer to a struct whose body is already being written stops at the
	// tag; anything else expands.
	if r, ok := elem.(*types.Record); ok {
		e.encodeRecord(b, r, !e.active(r))
		return
	}
	e.encode(b, p.Elem, false)
}

// encodeObject writes an object pointer: '@' in the plain encoding, and the
// class or protocols in the extended one.
func (e *encoder) encodeObject(b *strings.Builder, o *types.Object) {
	switch {
	case o.Meta:
		b.WriteByte('#')
		return
	case o.Instancetype:
		b.WriteByte('@')
		return
	}
	b.WriteByte('@')
	if !e.extended {
		return
	}
	switch {
	case o.Base != nil:
		b.WriteString(`"` + o.Base.Name + `"`)
	case len(o.Protocols) > 0:
		var names []string
		for _, p := range o.Protocols {
			names = append(names, "<"+p.Name+">")
		}
		b.WriteString(`"` + strings.Join(names, "") + `"`)
	}
}

// encodeRecord writes a struct or union, with or without its members.
//
// The name is the tag, or '?' where there is none — `{?=i}` is what an
// anonymous struct encodes as, and a program that reads the encoding back
// has no way to know what it was called because nothing called it anything.
func (e *encoder) encodeRecord(b *strings.Builder, r *types.Record, withBody bool) {
	open, close := byte('{'), byte('}')
	if r.Union {
		open, close = '(', ')'
	}
	b.WriteByte(open)
	name := r.Name
	if name == "" {
		name = "?"
	}
	b.WriteString(name)

	if withBody && r.Complete {
		b.WriteByte('=')
		e.inProgress = append(e.inProgress, r)
		for _, f := range r.Fields {
			if f.BitField {
				b.WriteByte('b')
				b.WriteString(strconv.FormatInt(f.Width, 10))
				continue
			}
			e.encode(b, f.Type, false)
		}
		e.inProgress = e.inProgress[:len(e.inProgress)-1]
	}
	b.WriteByte(close)
}

// active reports whether a record's body is already being written.
func (e *encoder) active(r *types.Record) bool {
	for _, x := range e.inProgress {
		if x == r {
			return true
		}
	}
	return false
}

// basicCode is objc4's alphabet for the builtin types.
//
// The letters are not mnemonic and not ordered: 'i' is int and 'I' unsigned
// int, but 'q' is long long and 'Q' unsigned long long, while 'l' and 'L' are
// long — which on a 64-bit target is the same width as 'q' and encoded
// differently anyway, because the encoding records the type the program
// wrote rather than the width it happened to have.
func basicCode(k types.Kind) string {
	switch k {
	case types.Void:
		return "v"
	case types.Bool:
		return "B"
	case types.Char:
		return "c"
	case types.SChar:
		return "c"
	case types.UChar:
		return "C"
	case types.Short:
		return "s"
	case types.UShort:
		return "S"
	case types.Int:
		return "i"
	case types.UInt:
		return "I"
	case types.Long:
		return "q"
	case types.ULong:
		return "Q"
	case types.LongLong:
		return "q"
	case types.ULongLong:
		return "Q"
	case types.Int128:
		return "t"
	case types.UInt128:
		return "T"
	case types.Float16:
		// clang encodes _Float16 as nothing at all, which was read off a
		// compiled @encode(_Float16). An empty encoding is what the
		// runtime gets, and there is no other answer to give it.
		return ""
	case types.Float:
		return "f"
	case types.Double:
		return "d"
	case types.LongDouble:
		return "D"
	}
	return "?"
}
