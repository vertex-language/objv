package runtime

// The structures the runtime walks.
//
// Each is described here as a field list with sizes rather than as a Go
// struct, because what lower needs is not a value of the type — it never
// holds one — but the order and width of what it writes. A field list is
// also what can be read against objc4's headers, which is the only way to
// know any of this is right.
//
// Every size below is for a 64-bit target. The runtime's own headers write
// them in terms of pointers and uint32s, and so does this: Size() computes
// from ABI.PtrBytes, and the constants that do not depend on it are stated as
// numbers because they are numbers in the metadata too — a method list's
// entry size is a uint32 field the runtime reads, not something it derives.

// FieldKind is what one field of a metadata structure holds.
type FieldKind uint8

const (
	// Ptr is a pointer, to a symbol or to nothing.
	Ptr FieldKind = iota
	// U32 and U64 are integers of that width.
	U32
	U64
	// Pad is padding the runtime does not read but the layout requires:
	// class_ro_t has four bytes of it on a 64-bit target, and a structure
	// written without them is one field short from there on.
	Pad
)

// Field is one field of a metadata structure: what it holds, and what it is
// called in objc4's headers, so that a reader can find it there.
type Field struct {
	Kind FieldKind
	Name string
}

// Size is a field's width in bytes.
func (a ABI) Size(f Field) int64 {
	switch f.Kind {
	case U32, Pad:
		return 4
	case U64:
		return 8
	}
	return a.ptr()
}

// SizeOf is the total size of a structure described as a field list.
func (a ABI) SizeOf(fields []Field) int64 {
	var n int64
	for _, f := range fields {
		n += a.Size(f)
	}
	return n
}

// OffsetOf is a named field's offset in a structure, and whether it is
// there. The metadata has no padding beyond what Pad states, so the offsets
// are a running sum.
func (a ABI) OffsetOf(fields []Field, name string) (int64, bool) {
	var n int64
	for _, f := range fields {
		if f.Name == name {
			return n, true
		}
		n += a.Size(f)
	}
	return 0, false
}

// Class is objc_class: what a class object is at run time.
//
// The first two fields are what dispatch reads — isa to find the metaclass,
// superclass to walk up — and the last is everything the compiler decided,
// which the runtime copies into a writable half the first time the class is
// used. `bits` is that pointer with flags in its low bits; the compiler
// writes the class_ro_t and nothing else.
var Class = []Field{
	{Ptr, "isa"},
	{Ptr, "superclass"},
	{Ptr, "cache"},
	{Ptr, "vtable"},
	{Ptr, "bits"}, // the class_ro_t, at compile time
}

// ClassRO is class_ro_t: the half of a class that never changes.
//
// instanceStart and instanceSize are why the non-fragile ABI works. The
// compiler writes what it believed the layout was; the runtime compares
// instanceStart against the superclass's real size and slides every
// instance variable by the difference, rewriting the offset variables as it
// goes. A superclass that grew is a number that differs, not a program that
// has to be rebuilt.
var ClassRO = []Field{
	{U32, "flags"},
	{U32, "instanceStart"},
	{U32, "instanceSize"},
	{Pad, "reserved"}, // 64-bit only; the runtime reads no field here
	{Ptr, "ivarLayout"},
	{Ptr, "name"},
	{Ptr, "baseMethodList"},
	{Ptr, "baseProtocols"},
	{Ptr, "ivars"},
	{Ptr, "weakIvarLayout"},
	{Ptr, "baseProperties"},
}

// Method is method_t: a selector, its type encoding, and the function.
//
// The name field holds a pointer to the *string* in the compiled image; the
// runtime replaces it with the unique SEL when it registers the method,
// which is why a selector comparison is a pointer comparison afterwards.
var Method = []Field{
	{Ptr, "name"},
	{Ptr, "types"},
	{Ptr, "imp"},
}

// Ivar is ivar_t.
//
// offset is a pointer to the offset *variable*, not the offset: the runtime
// writes through it when it realizes the class, and every access in every
// image loads it. alignment is a log2, and 255 means "unknown", which the
// compiler never writes.
var Ivar = []Field{
	{Ptr, "offset"},
	{Ptr, "name"},
	{Ptr, "type"},
	{U32, "alignment"},
	{U32, "size"},
}

// Property is property_t: a name and the attribute string that describes
// everything else about it.
var Property = []Field{
	{Ptr, "name"},
	{Ptr, "attributes"},
}

// Protocol is protocol_t.
//
// The size field near the end is the runtime's version check: an older
// runtime reads the fields it knows and stops, so a protocol grown a field
// stays loadable. It is written as the structure's own size.
var Protocol = []Field{
	{Ptr, "isa"}, // always null in the compiled image
	{Ptr, "name"},
	{Ptr, "protocols"},
	{Ptr, "instanceMethods"},
	{Ptr, "classMethods"},
	{Ptr, "optionalInstanceMethods"},
	{Ptr, "optionalClassMethods"},
	{Ptr, "instanceProperties"},
	{U32, "size"},
	{U32, "flags"},
	{Ptr, "extendedMethodTypes"},
	{Ptr, "demangledName"},
	{Ptr, "classProperties"},
}

// Category is category_t. Its own size is written into it for the same
// reason a protocol's is.
var Category = []Field{
	{Ptr, "name"},
	{Ptr, "cls"},
	{Ptr, "instanceMethods"},
	{Ptr, "classMethods"},
	{Ptr, "protocols"},
	{Ptr, "instanceProperties"},
	{Ptr, "classProperties"},
	{U32, "size"},
	{Pad, "reserved"},
}

// ImageInfo is the two words every image carries: a version and the flags
// that say what the compiler did.
var ImageInfo = []Field{
	{U32, "version"},
	{U32, "flags"},
}

// EntSize is the entsize a list of these entries carries in its header.
//
// A method list, an ivar list and a property list all begin with a uint32
// entsize and a uint32 count, and the runtime iterates by adding entsize
// rather than by sizeof: a list written by a newer compiler with wider
// entries is still walkable by an older runtime.
func (a ABI) EntSize(entry []Field) int64 { return a.SizeOf(entry) }

// ListHeader is the two words at the front of a method, ivar or property
// list.
var ListHeader = []Field{
	{U32, "entsize"},
	{U32, "count"},
}

// ProtocolListHeader is the one word at the front of a protocol list, which
// is a pointer-width count rather than the two uint32s the others use — and
// the list is null-terminated besides. Neither is a mistake to correct; both
// are what the runtime reads.
var ProtocolListHeader = []Field{
	{U64, "count"},
}

// ConstantString is the object a @"…" literal compiles to.
//
// On Darwin it is a CFString, not an NSString: the isa is
// __CFConstantStringClassReference, which CoreFoundation defines and objc4
// bridges, and that is why a constant string works in a process that never
// loaded Foundation. The four bytes after flags are padding on a 64-bit
// target, stated here for the same reason class_ro_t's are.
//
// length counts code units and not bytes — characters for an 8-bit string,
// UTF-16 units for a wide one — and the data pointer is NUL-terminated in
// both encodings.
var ConstantString = []Field{
	{Ptr, "isa"},
	{U32, "flags"},
	{Pad, "reserved"},
	{Ptr, "data"},
	{U64, "length"},
}

// The flags word of a constant string, which says how data is encoded.
//
// The values are CoreFoundation's own and are not derived from anything: the
// low bits describe an immutable, constant, not-inline CFString, and the one
// that varies is whether the bytes are 8-bit or UTF-16. clang emits exactly
// these two, which is what they were read off.
const (
	// CFStringASCII is an 8-bit string, in __TEXT,__cstring.
	CFStringASCII uint32 = 0x7C8
	// CFStringUTF16 is a UTF-16 string, in __TEXT,__ustring.
	CFStringUTF16 uint32 = 0x7D0
)

// FastEnumerationState is NSFastEnumerationState: the scratch a fast
// enumeration loop hands the collection and the collection fills in.
//
// The compiler declares it, zeroes it, and passes its address; everything in
// it belongs to the collection. state is the collection's own cursor,
// itemsPtr is where it left this batch of elements — which may be the
// buffer the loop supplied or storage of the collection's own — and
// mutationsPtr addresses the counter the loop watches. extra is five words
// the collection may use for anything.
var FastEnumerationState = []Field{
	{U64, "state"},
	{Ptr, "itemsPtr"},
	{Ptr, "mutationsPtr"},
	{U64, "extra0"},
	{U64, "extra1"},
	{U64, "extra2"},
	{U64, "extra3"},
	{U64, "extra4"},
}

// FastEnumerationBatch is how many elements a loop asks for at a time. It is
// the compiler's choice and not the runtime's: the buffer is the loop's, and
// sixteen is what clang picks.
const FastEnumerationBatch = 16

// EHType is the type-info object a @catch names: Itanium's layout, with the
// class hanging off the end of it.
//
// The vtable pointer is the odd field. Itanium's points at the first virtual
// function rather than at the two-word header above it, so what goes here is
// objc_ehtype_vtable plus EHTypeVTableOffset — the runtime's own object has
// the same shape and the personality reads both through the same code.
var EHType = []Field{
	{Ptr, "vtable"},
	{Ptr, "name"},
	{Ptr, "cls"},
}
