package runtime

// Symbols and prefixes used in Objective-C metadata emission and runtime linking.
//
// Key prefixes:
//   - OBJC_CLASS_$_: class symbols resolved across images
//   - _OBJC_$_: metadata symbols internal to the image
//   - l_OBJC_: local labels preserved in assembly but omitted from linker export

// ClassSymbol returns the symbol for a class object.
func ClassSymbol(class string) string { return "OBJC_CLASS_$_" + class }

// EHTypeSymbol returns the type-info symbol for a class used in exception matching.
func EHTypeSymbol(class string) string { return "OBJC_EHTYPE_$_" + class }

// MetaclassSymbol returns the metaclass symbol holding class methods.
func MetaclassSymbol(class string) string { return "OBJC_METACLASS_$_" + class }

// ClassROSymbol and MetaclassROSymbol return read-only class/metaclass metadata symbols.
func ClassROSymbol(class string) string     { return "_OBJC_CLASS_RO_$_" + class }
func MetaclassROSymbol(class string) string { return "_OBJC_METACLASS_RO_$_" + class }

// IvarOffsetSymbol returns the variable holding an instance variable's offset (non-fragile ABI).
func IvarOffsetSymbol(class, ivar string) string {
	return "OBJC_IVAR_$_" + class + "." + ivar
}

// The method, ivar and property lists of a class.
func InstanceMethodsSymbol(class string) string { return "_OBJC_$_INSTANCE_METHODS_" + class }
func ClassMethodsSymbol(class string) string    { return "_OBJC_$_CLASS_METHODS_" + class }
func IvarsSymbol(class string) string           { return "_OBJC_$_INSTANCE_VARIABLES_" + class }
func PropertiesSymbol(class string) string      { return "_OBJC_$_PROP_LIST_" + class }
func ClassPropertiesSymbol(class string) string { return "_OBJC_$_CLASS_PROP_LIST_" + class }

// ClassProtocolsSymbol is the protocol list a class conforms to.
func ClassProtocolsSymbol(class string) string { return "_OBJC_CLASS_PROTOCOLS_$_" + class }

// ProtocolSymbol is the protocol object.
//
// It is emitted weak and coalesced: every image that mentions a protocol
// defines it, and the linker keeps one. That is how a protocol declared in a
// header shared by ten frameworks is one protocol at run time.
func ProtocolSymbol(protocol string) string { return "_OBJC_PROTOCOL_$_" + protocol }

// ProtocolLabelSymbol is the entry in the protocol list — a pointer to the
// protocol object, in its own coalesced symbol so the linker can drop the
// duplicates.
func ProtocolLabelSymbol(protocol string) string { return "_OBJC_LABEL_PROTOCOL_$_" + protocol }

// A protocol's four method lists, and the extended type list beside them.
func ProtocolInstanceMethodsSymbol(p string) string { return "_OBJC_$_PROTOCOL_INSTANCE_METHODS_" + p }
func ProtocolClassMethodsSymbol(p string) string    { return "_OBJC_$_PROTOCOL_CLASS_METHODS_" + p }
func ProtocolOptionalInstanceMethodsSymbol(p string) string {
	return "_OBJC_$_PROTOCOL_INSTANCE_METHODS_OPT_" + p
}
func ProtocolOptionalClassMethodsSymbol(p string) string {
	return "_OBJC_$_PROTOCOL_CLASS_METHODS_OPT_" + p
}
func ProtocolPropertiesSymbol(p string) string  { return "_OBJC_$_PROP_LIST_" + p }
func ProtocolMethodTypesSymbol(p string) string { return "_OBJC_$_PROTOCOL_METHOD_TYPES_" + p }
func ProtocolRefsSymbol(p string) string        { return "_OBJC_$_PROTOCOL_REFS_" + p }

// CategorySymbol and its lists. A category is named for the class it
// extends and the name it was given, because two categories on one class are
// two pieces of metadata and the runtime attaches both.
func CategorySymbol(class, category string) string {
	return "_OBJC_$_CATEGORY_" + class + "_$_" + category
}

func CategoryInstanceMethodsSymbol(class, category string) string {
	return "_OBJC_$_CATEGORY_INSTANCE_METHODS_" + class + "_$_" + category
}

func CategoryClassMethodsSymbol(class, category string) string {
	return "_OBJC_$_CATEGORY_CLASS_METHODS_" + class + "_$_" + category
}

func CategoryProtocolsSymbol(class, category string) string {
	return "_OBJC_CATEGORY_PROTOCOLS_$_" + class + "_$_" + category
}

func CategoryPropertiesSymbol(class, category string) string {
	return "_OBJC_$_PROP_LIST_" + class + "_$_" + category
}

// MethodName formats a method's display name, e.g. "-[NSString length]".
func MethodName(class, category, sel string, classMethod bool) string {
	sign := "-"
	if classMethod {
		sign = "+"
	}
	if category != "" {
		return sign + "[" + class + "(" + category + ") " + sel + "]"
	}
	return sign + "[" + class + " " + sel + "]"
}

// MethodSymbol formats the mangled symbol name for a method's function
// following the GNU/libobjc2 convention (e.g. "_i_NSString__length").
func MethodSymbol(class, category, sel string, classMethod bool) string {
	kind := "_i_"
	if classMethod {
		kind = "_c_"
	}
	mangled := make([]byte, 0, len(sel))
	for i := 0; i < len(sel); i++ {
		if sel[i] == ':' {
			mangled = append(mangled, '_')
			continue
		}
		mangled = append(mangled, sel[i])
	}
	return kind + class + "_" + category + "_" + string(mangled)
}

// The labels a translation unit's own lists are gathered under. Each is a
// list of pointers the runtime walks at load: one entry per class, category
// or protocol the image defines.
const (
	ClassListLabel    = "l_OBJC_LABEL_CLASS_$"
	CategoryListLabel = "l_OBJC_LABEL_CATEGORY_$"
	ImageInfoLabel    = "L_OBJC_IMAGE_INFO"

	// And the non-lazy lists, which hold the entries that implement +load.
	NonLazyClassListLabel    = "l_OBJC_LABEL_NONLAZY_CLASS_$"
	NonLazyCategoryListLabel = "l_OBJC_LABEL_NONLAZY_CATEGORY_$"

	// LoadSelector is the one selector the runtime sends without being
	// asked: every class and category that implements it is sent +load
	// when the image is mapped, before main and before any message.
	LoadSelector = "load"
)

// The references a translation unit makes, which the runtime rewrites at
// load: a selector reference becomes the unique SEL for that name, a class
// reference becomes the class object.
//
// They are per-name, and the emitter is expected to keep one of each: two
// sends of the same selector in one file share one selector reference, which
// is what makes a send two instructions rather than a lookup.
const (
	SelectorRefPrefix = "OBJC_SELECTOR_REFERENCES_"
	ClassRefPrefix    = "OBJC_CLASSLIST_REFERENCES_$_"
	SuperRefPrefix    = "l_OBJC_CLASSLIST_SUP_REFS_$_"
)

// The string labels. The runtime does not read these names; the assembler
// needs one per string, and a reader of the output should be able to tell a
// selector from a class name at a glance.
const (
	ConstStringLabel  = "l_unnamed_cfstring_"
	CStringLabel      = "l_.str"
	ClassNameLabel    = "l_OBJC_CLASS_NAME_"
	MethodNameLabel   = "l_OBJC_METH_VAR_NAME_"
	MethodTypeLabel   = "l_OBJC_METH_VAR_TYPE_"
	PropertyAttrLabel = "l_OBJC_PROP_NAME_ATTR_"
)

// The runtime entry points a lowered translation unit calls. Naming them
// here rather than at each call site is what keeps a typo from becoming an
// undefined symbol at link time.
const (
	MsgSend       = "objc_msgSend"
	MsgSendStret  = "objc_msgSend_stret"
	MsgSendFpret  = "objc_msgSend_fpret"
	MsgSendFp2ret = "objc_msgSend_fp2ret"

	// MsgSendSuper2 invokes objc_msgSendSuper2 (takes class rather than superclass).
	MsgSendSuper2      = "objc_msgSendSuper2"
	MsgSendSuper2Stret = "objc_msgSendSuper2_stret"

	// ARC runtime entry points.
	Retain                        = "objc_retain"
	Release                       = "objc_release"
	Autorelease                   = "objc_autorelease"
	RetainAutorelease             = "objc_retainAutorelease"
	RetainAutoreleasedReturnValue = "objc_retainAutoreleasedReturnValue"
	AutoreleaseReturnValue        = "objc_autoreleaseReturnValue"
	StoreStrong                   = "objc_storeStrong"
	StoreWeak                     = "objc_storeWeak"
	LoadWeakRetained              = "objc_loadWeakRetained"
	LoadWeak                      = "objc_loadWeak"
	InitWeak                      = "objc_initWeak"
	DestroyWeak                   = "objc_destroyWeak"
	CopyWeak                      = "objc_copyWeak"

	// CxxDestructSelector is called during deallocation to release ivars.
	CxxDestructSelector = ".cxx_destruct"

	AutoreleasePoolPush = "objc_autoreleasePoolPush"
	AutoreleasePoolPop  = "objc_autoreleasePoolPop"

	// GetProperty is called by synthesized getters for atomic properties.
	GetProperty = "objc_getProperty"

	// RetainBlock retains a block, copying stack blocks to the heap.
	RetainBlock = "objc_retainBlock"

	// Exceptions (§7.2) and synchronization (§7.3).
	ExceptionThrow   = "objc_exception_throw"
	ExceptionRethrow = "objc_exception_rethrow"
	SyncEnter        = "objc_sync_enter"
	SyncExit         = "objc_sync_exit"

	// Personality is the unwinder routine for frames containing @try blocks.
	Personality = "__objc_personality_v0"

	// BeginCatch and EndCatch manage exception handler scope.
	BeginCatch = "objc_begin_catch"
	EndCatch   = "objc_end_catch"

	// EHTypeVTable is the vtable every type-info object points at, and the
	// pointer is to its third word rather than to its first: the layout is
	// Itanium's, whose vtable pointer names the first virtual function and
	// not the header two words above it.
	EHTypeVTable       = "objc_ehtype_vtable"
	EHTypeVTableOffset = 16

	// EHTypeID is the type-info `@catch (id)` names — the one that matches
	// any Objective-C object. A `@catch (...)` names nothing at all, which
	// the table spells as a null type-info.
	EHTypeID = "OBJC_EHTYPE_id"

	// Fast enumeration (§7.1) is a method, not a function: the loop sends
	// this selector to the collection.
	FastEnumerationSelector = "countByEnumeratingWithState:objects:count:"

	// EnumerationMutation is what a fast-enumeration loop calls when the
	// collection changed under it. The loop compares a counter the
	// collection publishes before and after each element, and this is the
	// call that turns a disagreement into a diagnosed crash rather than a
	// walk off the end of a stale buffer.
	EnumerationMutation = "objc_enumerationMutation"

	// The empty cache every class points at until the runtime gives it one.
	EmptyCache = "_objc_empty_cache"
)

// ConstantStringClass is the isa a @"…" literal is given.
//
// On Darwin it is CoreFoundation's, not Foundation's: a constant string is a
// CFString that objc4 bridges, which is what lets one exist in a process that
// never loaded Foundation and why the symbol has nothing to do with NSString.
// libobjc2 has no such bridge and uses its own class.
func (a ABI) ConstantStringClass() string {
	if a.Kind == GNUstep {
		return "_NSConstantStringClassReference"
	}
	return "__CFConstantStringClassReference"
}

// SetPropertySymbol is the runtime entry a synthesized setter calls, of which
// there are four: the two axes are whether the store is atomic and whether
// the value is copied.
//
//	void objc_setProperty_nonatomic_copy(id self, SEL _cmd, id value, ptrdiff_t offset);
//
// They are separate symbols rather than one function with two flags because
// that is what clang calls and what the runtime exports; the generic
// objc_setProperty exists too and takes the flags, and nothing gains by
// using it.
func SetPropertySymbol(atomic, copy bool) string {
	name := "objc_setProperty_nonatomic"
	if atomic {
		name = "objc_setProperty_atomic"
	}
	if copy {
		name += "_copy"
	}
	return name
}

// The C runtime's two names for static initialization and finalization.
//
// A constructor needs no name — the image's __mod_init_func names it by
// address, and dyld calls what is there. A destructor does: nothing walks a
// list of them. clang registers each one with the C++ ABI's at-exit, from
// inside a synthesized initializer, and so does this; the handle identifies
// the image, so that unloading a bundle runs the destructors that came with
// it and no others.
//
//	int __cxa_atexit(void (*f)(void *), void *arg, void *dso);
const (
	CxaAtexit = "__cxa_atexit"
	DsoHandle = "__dso_handle"

	// StaticInitFunc is the name clang gives the initializer it synthesizes
	// to hold those registrations, priority and all. Nothing requires the
	// name — it is internal, and the list names it by address — but a stack
	// trace through an at-exit registration reads better with the name the
	// platform's other compiler uses.
	StaticInitFunc = "__GLOBAL_init_"
)
