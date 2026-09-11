package runtime

// The block ABI.
//
// A block is not part of the Objective-C runtime — it is C's, and it works
// in a .c file with no classes anywhere near it — but it is described here
// with the class metadata because it is the same kind of fact: a layout two
// separate programs have to agree on, where one of them is libSystem and
// cannot be changed. What follows is Apple's Block-ABI-Apple.txt, checked
// against what clang emits, which is the only way to know it is right.
//
// A block literal is a structure whose first word is an isa: a block *is* an
// object, which is why it can be sent -copy and put in an NSArray. The rest
// is a function pointer and a pointer to a descriptor, and then whatever the
// literal captured, laid out in the order the literal captured it.
//
//	struct block_literal {
//	    void *isa;              // _NSConcreteStackBlock or _NSConcreteGlobalBlock
//	    int32_t flags;
//	    int32_t reserved;
//	    void (*invoke)(void *, ...);
//	    struct block_descriptor *descriptor;
//	    // captures
//	};
//
// Calling one is `b->invoke(b, args...)`: the block passes itself as the
// hidden first argument, which is how the invoke function reaches the
// captures.

// BlockFlag is the flags word of a block literal. Only the ones objv writes
// or reads are named; the rest of the word belongs to the runtime, which
// keeps a reference count in the low 16 bits of a block it has copied.
type BlockFlag uint32

const (
	// BlockHasCopyDispose says the descriptor carries the two helper
	// functions, which the runtime calls when a block is copied to the heap
	// and when the copy dies. A block that captured an object has them and a
	// block that captured only scalars does not: there is nothing to retain.
	BlockHasCopyDispose BlockFlag = 1 << 25

	// BlockIsGlobal says the literal is a global rather than a stack
	// object. _Block_copy of one returns it unchanged — there is nothing to
	// copy, since it captured nothing and so cannot differ between
	// executions of the statement that named it.
	BlockIsGlobal BlockFlag = 1 << 28

	// BlockHasStret says invoke takes a hidden return buffer, so the
	// runtime's forwarding must call it through the struct-return
	// convention.
	BlockHasStret BlockFlag = 1 << 29

	// BlockHasSignature says the descriptor carries the @encode string for
	// invoke. Everything modern sets it: it is what lets the runtime build
	// an NSMethodSignature for a block, and clang sets it unconditionally.
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

// What a copy or dispose helper says it is handling. The runtime's
// BLOCK_FIELD_IS_* values: an object is retained and released, a block is
// copied and released, and a __block variable — BLOCK_FIELD_IS_BYREF, which
// objv does not emit yet — is a structure with a refcount of its own.
const (
	BlockFieldObject = 3 // BLOCK_FIELD_IS_OBJECT
	BlockFieldBlock  = 7 // BLOCK_FIELD_IS_BLOCK
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

// The two labels a block's data carries. Both are internal: nothing outside
// the image names either.
const (
	BlockDescriptorLabel = "__block_descriptor_tmp"
	BlockLiteralLabel    = "__block_literal_global"
)
