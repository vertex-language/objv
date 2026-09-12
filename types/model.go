package types

// Model defines a target's type model (sizes, alignments, signedness).
// Layout (Sizeof, Alignof, field offsets) is determined purely by the Model.
type Model struct {
	CharSigned bool
	WCharKind  Kind // the basic kind wchar_t aliases

	SizeShort, SizeInt, SizeLong, SizeLongLong int64
	SizePtr                                    int64
	SizeFloat, SizeDouble, SizeLongDouble      int64
	AlignLongDouble                            int64

	// MaxVectorAlign is the maximum vector alignment (register width, typically 16 bytes).
	MaxVectorAlign int64

	// MSBitfields selects Microsoft bit-field layout rules (Windows ABI).
	MSBitfields bool

	// ObjCBoolIsBool indicates BOOL is bool rather than signed char (arm64 Apple ABI).
	ObjCBoolIsBool bool

	// VaListSize is the byte size of __builtin_va_list (0 means pointer size).
	VaListSize int64
}

// LP64 returns the standard 64-bit target type model.
func LP64() Model {
	return Model{
		CharSigned: true,
		WCharKind:  Int,
		SizeShort:  2, SizeInt: 4, SizeLong: 8, SizeLongLong: 8,
		SizePtr:   8,
		SizeFloat: 4, SizeDouble: 8, SizeLongDouble: 16,
		AlignLongDouble: 16,
		MaxVectorAlign:  16,
	}
}

// Sizeof returns a type's size in bytes. ok is false for incomplete
// types, function types, and VLAs — sizes sizeof cannot know.
func (m Model) Sizeof(t Type) (int64, bool) {
	switch t := Unqualify(t).(type) {
	case *Basic:
		return m.basicSize(t.K)
	case *Pointer, *Block:
		return m.SizePtr, true
	case *TypeParam:
		// A type parameter is erased: a value of one is an object pointer.
		return m.SizePtr, true
	case *Enum:
		sz, _ := m.basicSize(t.Underlying())
		return sz, t.Complete
	case *Array:
		if t.Form != FixedArray {
			return 0, false
		}
		e, ok := m.Sizeof(t.Elem)
		return e * t.Len, ok
	case *Vector:
		return m.vectorSize(t)
	case *Record:
		if !t.Complete {
			return 0, false
		}
		size, _, ok := m.layout(t, nil)
		return size, ok
	}
	return 0, false
}

func (m Model) basicSize(k Kind) (int64, bool) {
	switch k {
	case Void:
		return 0, false
	case Bool, Char, SChar, UChar:
		return 1, true
	case Short, UShort:
		return m.SizeShort, true
	case Int, UInt:
		return m.SizeInt, true
	case Long, ULong:
		return m.SizeLong, true
	case LongLong, ULongLong:
		return m.SizeLongLong, true
	// __int128 and _Float16 are the same size everywhere they exist; there
	// is no target model that gives them another, so they are stated rather
	// than carried as fields nothing would ever set differently.
	case Int128, UInt128:
		return 16, true
	case Float16:
		return 2, true
	case Float:
		return m.SizeFloat, true
	case Double:
		return m.SizeDouble, true
	case LongDouble:
		return m.SizeLongDouble, true
	case ComplexFloat:
		return 2 * m.SizeFloat, true
	case ComplexDouble:
		return 2 * m.SizeDouble, true
	case ComplexLongDouble:
		return 2 * m.SizeLongDouble, true
	}
	return 0, false
}

// Alignof returns a type's alignment requirement.
func (m Model) Alignof(t Type) (int64, bool) {
	switch t := Unqualify(t).(type) {
	case *Basic:
		if t.K == LongDouble || t.K == ComplexLongDouble {
			return m.AlignLongDouble, true
		}
		if t.K == ComplexFloat {
			return m.SizeFloat, true
		}
		if t.K == ComplexDouble {
			return m.SizeDouble, true
		}
		return m.basicSize(t.K)
	case *Pointer, *Block, *TypeParam:
		return m.SizePtr, true
	case *Enum:
		sz, _ := m.basicSize(t.Underlying())
		return sz, t.Complete
	case *Array:
		return m.Alignof(t.Elem)
	case *Vector:
		// A vector aligns to its own size, up to the width of a vector
		// register: a simd_float2 is eight bytes and aligns to eight, a
		// simd_float4 is sixteen and aligns to sixteen, and a simd_float16
		// is sixty-four and still aligns to sixteen, because the machine
		// loads it in four pieces and each piece is what has to be aligned.
		sz, ok := m.vectorSize(t)
		if !ok {
			return 0, false
		}
		if sz > m.MaxVectorAlign {
			return m.MaxVectorAlign, true
		}
		return sz, true
	case *Record:
		if !t.Complete {
			return 0, false
		}
		_, align, ok := m.layout(t, nil)
		return align, ok
	}
	return 0, false
}

// vectorSize returns the storage size of a vector (rounded up to power of two).
func (m Model) vectorSize(t *Vector) (int64, bool) {
	e, ok := m.Sizeof(t.Elem)
	if !ok || t.Len <= 0 {
		return 0, false
	}
	n := int64(1)
	for n < t.Len {
		n *= 2
	}
	return e * n, true
}

// Offsetof returns the byte offset of a member from the base of a record.
// Returns false for incomplete records, non-existent members, or bit-fields.
func (m Model) Offsetof(t Type, name string) (int64, bool) {
	r, ok := Unqualify(t).(*Record)
	if !ok || !r.Complete {
		return 0, false
	}
	offs := make([]int64, len(r.Fields))
	if _, _, ok := m.layoutWith(r, offs, nil); !ok {
		return 0, false
	}
	for i, f := range r.Fields {
		if f.Name == name {
			if f.BitField {
				return 0, false
			}
			return offs[i], true
		}
	}
	for i, f := range r.Fields {
		if f.Name != "" {
			continue
		}
		if _, ok := Unqualify(f.Type).(*Record); !ok {
			continue
		}
		if n, ok := m.Offsetof(f.Type, name); ok {
			return offs[i] + n, true
		}
	}
	return 0, false
}

// layout computes a record's size and alignment, and optional member byte offsets.
func (m Model) layout(r *Record, offs []int64) (size, align int64, ok bool) {
	return m.layoutWith(r, offs, nil)
}

// BitPlaces computes bit-field layout information for a record's fields.
func (m Model) BitPlaces(t Type) ([]BitPlace, bool) {
	r, ok := Unqualify(t).(*Record)
	if !ok || !r.Complete {
		return nil, false
	}
	places := make([]BitPlace, len(r.Fields))
	if _, _, ok := m.layoutWith(r, nil, places); !ok {
		return nil, false
	}
	return places, true
}

func (m Model) layoutWith(r *Record, offs []int64, places []BitPlace) (size, align int64, ok bool) {
	align = 1
	var cur BitCursor // the placement cursor; see bitfield.go
	for i, f := range r.Fields {
		fs, ok1 := m.Sizeof(f.Type)
		if !ok1 {
			// The flexible array member contributes no size. It is the last
			// member of a struct, and any member of a union — where there
			// is no order for it to be last in.
			if a, ok := f.Type.(*Array); ok && a.Form == IncompleteArray &&
				(r.Union || i == len(r.Fields)-1) {
				ok1 = true
				fs = 0
			}
		}
		fa, ok2 := m.Alignof(f.Type)
		if !ok1 || !ok2 {
			return 0, 0, false
		}
		// __attribute__((packed)) flattens every member's alignment to one;
		// #pragma pack caps it. Record.MemberAlign is the rule, stated once
		// so that this layout and lower's cannot drift.
		natAl := fa
		fa = r.MemberAlign(fa)
		// An unnamed bit-field carries storage but no alignment: it cannot
		// raise the record's. lower/layout.go states the same rule and the
		// two must agree, or layout warns about itself.
		if fa > align && (!f.BitField || f.Name != "") {
			align = fa
		}
		if r.Union {
			// Every member of a union starts at its base.
			if offs != nil {
				offs[i] = 0
			}
			// A bit-field member is as wide as its bits, not as wide as its
			// declared type.
			w := fs * 8
			if f.BitField {
				w = roundUp(f.Width, 8)
			}
			if w > cur.Bits {
				cur.Bits = w
			}
			continue
		}
		if f.BitField {
			switch {
			case f.Width == 0:
				cur.ZeroWidth(m.MSBitfields, natAl)
			case r.Packed:
				// __attribute__((packed)) admits no padding at all, so the
				// field goes at the next bit whatever its declared type is.
				// It is the one case neither allocation rule describes.
				cur.CloseUnit()
				if places != nil {
					// The unit is the byte the field starts in, and it is as
					// wide as the field needs: packed means there is no
					// allocation unit left to speak of.
					start := cur.Bits
					places[i] = BitPlace{
						Off:    start / 8,
						BitOff: start % 8,
						Unit:   roundUp(start%8+f.Width, 8) / 8,
					}
				}
				cur.Bits += f.Width
			default:
				p := cur.PlaceBitfield(m.MSBitfields, fs, fa, f.Width)
				if places != nil {
					places[i] = p
				}
			}
			continue
		}
		off := cur.CloseForMember()
		off = roundUp(off, fa)
		if offs != nil {
			offs[i] = off
		}
		cur.Bits = (off + fs) * 8
	}
	bits := cur.End()
	// __attribute__((aligned(n))) raises the record's alignment, and with it
	// the size, whether or not the record is packed: the two attributes
	// answer different questions.
	if r.Align > align {
		align = r.Align
	}
	size = roundUp(bits, align*8) / 8
	if size == 0 {
		size = 0 // an empty struct is a constraint violation reported elsewhere
	}
	return size, align, true
}

func roundUp(n, to int64) int64 {
	if to == 0 {
		return n
	}
	return (n + to - 1) / to * to
}

// IntBits returns the width in bits of an integer type, and whether
// values of it are signed — plain char resolved by the model.
func (m Model) IntBits(t Type) (bits int64, signed bool) {
	u := Unqualify(t)
	k := u.Kind()
	if e, ok := u.(*Enum); ok {
		k = e.Underlying()
	}
	sz, _ := m.basicSize(k)
	signed = IsSigned(Unqualify(t))
	if k == Char {
		signed = m.CharSigned
	}
	return sz * 8, signed
}

// IntMax returns the largest value an integer type can hold, as a
// uint64.
func (m Model) IntMax(t Type) uint64 {
	bits, signed := m.IntBits(t)
	if signed {
		return 1<<(bits-1) - 1
	}
	if bits >= 64 {
		return ^uint64(0)
	}
	return 1<<bits - 1
}

// FieldOffsets returns every member's byte offset in declaration order.
func (m Model) FieldOffsets(r *Record) ([]int64, bool) {
	if r == nil || !r.Complete {
		return nil, false
	}
	offs := make([]int64, len(r.Fields))
	if _, _, ok := m.layout(r, offs); !ok {
		return nil, false
	}
	return offs, true
}
