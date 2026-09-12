package runtime

// Block ABI definitions and struct layouts (per Apple Block-ABI-Apple.txt).
//
// A block literal begins with an isa pointer, flags, reserved, invoke function pointer,
// and block descriptor pointer, followed by captured variables.

// BlockFlag represents flags in the block literal header.
type BlockFlag uint32

const (
	// BlockHasCopyDispose indicates descriptor contains copy/dispose helper functions.
	BlockHasCopyDispose BlockFlag = 1 << 25

	// BlockIsGlobal indicates the block literal is global (constant in data).
	BlockIsGlobal BlockFlag = 1 << 28

	// BlockHasStret indicates the block uses struct-return calling conventions.
	BlockHasStret BlockFlag = 1 << 29

	// BlockHasSignature indicates the descriptor carries an @encode type signature.
	BlockHasSignature BlockFlag = 1 << 30
)

// BlockLiteral is the header every block carries, whatever it captured.
//
// flags and reserved are two 32-bit fields and not one 64-bit one, which
// matters on a big-endian target and is why they are written separately.
var BlockLiteral = []Field{
	{Ptr, "isa"},
	{U32, "flags"},
	{U32, "reserved"},
	{Ptr, "invoke"},
	{Ptr, "descriptor"},
}

// BlockDescriptor is the descriptor of a block with nothing to retain.
//
// The two trailing fields are Block_descriptor_3, present because
// BlockHasSignature is set; layout is the extended-layout string ARC uses to
// tell the runtime which captures are objects, and is null where objv has
// not computed one. The runtime reads it only when the flag for it is set,
// which objv does not set, so a null there is a null it never looks at.
var BlockDescriptor = []Field{
	{U64, "reserved"},
	{U64, "size"},
	{Ptr, "signature"},
	{Ptr, "layout"},
}

// BlockDescriptorWithHelpers is the descriptor of a block that captured
// something the runtime has to retain and release. The two helpers go
// between size and signature — Block_descriptor_2 sits there — which is why
// the descriptor cannot be one layout with optional fields at the end.
var BlockDescriptorWithHelpers = []Field{
	{U64, "reserved"},
	{U64, "size"},
	{Ptr, "copy"},
	{Ptr, "dispose"},
	{Ptr, "signature"},
	{Ptr, "layout"},
}

// The block runtime's symbols, spelled without the platform's leading
// underscore as everything else here is.
const (
	// StackBlockClass and GlobalBlockClass are the isa a literal is born
	// with. Neither is a class objv declares: both are objects in
	// libSystem, imported like any other data symbol.
	StackBlockClass  = "_NSConcreteStackBlock"
	GlobalBlockClass = "_NSConcreteGlobalBlock"

	// BlockObjectAssign and BlockObjectDispose are what a copy helper and a
	// dispose helper call, once per captured object.
	BlockObjectAssign  = "_Block_object_assign"
	BlockObjectDispose = "_Block_object_dispose"

	// BlockCopy moves a block to the heap and BlockRelease lets one go.
	BlockCopy    = "_Block_copy"
	BlockRelease = "_Block_release"
)

// BlockByref is the header layout for a __block variable, accessed via its
// forwarding pointer so accesses remain valid when migrated to the heap.
var BlockByref = []Field{
	{Ptr, "isa"},
	{Ptr, "forwarding"},
	{U32, "flags"},
	{U32, "size"},
}

// BlockByrefWithHelpers is the structure when the variable is something the
// runtime has to hand over rather than copy: the two helpers sit between the
// header and the variable, which is why this is a second layout and not the
// first with fields on the end.
var BlockByrefWithHelpers = []Field{
	{Ptr, "isa"},
	{Ptr, "forwarding"},
	{U32, "flags"},
	{U32, "size"},
	{Ptr, "byref_keep"},
	{Ptr, "byref_destroy"},
}

// The flags word of a byref. The high nibble is a layout describing what the
// variable is, and which one an object gets is the memory model's answer
// rather than the type's: a __block object is not retained under manual
// reference counting and is under ARC, which is the whole difference between
// the two spellings clang writes.
const (
	BlockByrefHasCopyDispose   BlockFlag = 1 << 25
	BlockByrefLayoutStrong     BlockFlag = 3 << 28
	BlockByrefLayoutWeak       BlockFlag = 4 << 28
	BlockByrefLayoutUnretained BlockFlag = 5 << 28
)

// What a copy or dispose helper says it is handling. The runtime's
// BLOCK_FIELD_IS_* values: an object is retained and released, a block is
// copied and released, and a __block variable — BLOCK_FIELD_IS_BYREF, which
// objv does not emit yet — is a structure with a refcount of its own.
const (
	BlockFieldObject = 3 // BLOCK_FIELD_IS_OBJECT
	BlockFieldBlock  = 7 // BLOCK_FIELD_IS_BLOCK
	BlockFieldByref  = 8 // BLOCK_FIELD_IS_BYREF

	// BlockByrefCaller is ored into the flag a *byref's own* helper passes,
	// to say the caller is the byref machinery rather than a block's copy
	// helper. The runtime's own tests read 0x83 out of clang's output for
	// an object __block, which is this plus BlockFieldObject.
	BlockByrefCaller = 128
)

// Block literal, descriptor and invoke labels.
//
// The names are clang's, because a backtrace through a block is read by
// people who know what ___main_block_invoke means. The enclosing function's
// name is in the middle, and a second literal in the same function gets a
// numbered suffix, which lower applies.
func BlockInvokeSymbol(fn string) string { return "__" + fn + "_block_invoke" }

// BlockCopySymbol and BlockDisposeSymbol name the two helpers.
func BlockCopySymbol(fn string) string    { return "__copy_helper_block_" + fn }
func BlockDisposeSymbol(fn string) string { return "__destroy_helper_block_" + fn }

// The byref's helpers. clang names them by what the variable is rather than
// by which variable it is, because one helper serves every byref with an
// object at the same offset.
func BlockByrefCopySymbol(n int) string    { return "__Block_byref_object_copy_" + itoa(n) }
func BlockByrefDisposeSymbol(n int) string { return "__Block_byref_object_dispose_" + itoa(n) }

// The two labels a block's data carries. Both are internal: nothing outside
// the image names either.
const (
	BlockDescriptorLabel = "__block_descriptor_tmp"
	BlockLiteralLabel    = "__block_literal_global"
)

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
