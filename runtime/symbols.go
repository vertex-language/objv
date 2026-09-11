package runtime

// The symbols a translation unit defines and references.
//
// Every name here is the one the runtime and the linker agree on, written
// without the platform's leading underscore: Mach-O adds one when the object
// writer emits the symbol, exactly as it does for a C function, and a name
// that carried it here would be wrong on ELF.
//
// Three prefixes carry meaning and are worth reading before the list:
//
//	OBJC_CLASS_$_       a class, which the linker resolves across images
//	_OBJC_$_            a piece of metadata private to this image
//	l_OBJC_             a label the assembler keeps and the linker does not
//
// The middle one is why so many names below begin with an underscore that
// looks redundant: it is the runtime's own convention for "this is not a
// symbol anybody outside links against", and objc4's tools key on it.

// ClassSymbol is the class object: what `[Foo class]` yields, what a
// subclass in another image links against, and what the class list points
// at.
func ClassSymbol(class string) string { return "OBJC_CLASS_$_" + class }

// EHTypeSymbol is a class's type-info object: three words the personality
// reads to decide whether a thrown object is one of these.
//
// It exists for a class only where something catches it, and it is global
// rather than local because the @catch may be in another image than the
// @implementation — which is why it is a symbol with the class's name in it
// rather than an anonymous constant.
func EHTypeSymbol(class string) string { return "OBJC_EHTYPE_$_" + class }

// MetaclassSymbol is the metaclass object, which holds the class methods.
// The metaclass of a root class is its own isa, which is what closes the
// chain.
func MetaclassSymbol(class string) string { return "OBJC_METACLASS_$_" + class }

// ClassROSymbol and MetaclassROSymbol are the read-only halves: everything
// about a class that is decided at compile time, which the runtime copies
// out of and never writes to.
func ClassROSymbol(class string) string     { return "_OBJC_CLASS_RO_$_" + class }
func MetaclassROSymbol(class string) string { return "_OBJC_METACLASS_RO_$_" + class }

// IvarOffsetSymbol is the variable holding one instance variable's offset.
//
// It is the whole of the non-fragile ABI in one symbol: the offset is not a
// constant in the instruction stream but a global the runtime writes when
// the class is realized, so a superclass may grow an instance variable
// without every subclass having to be recompiled. Every ivar access loads
// this and adds it.
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

// MethodName is what a method is *called*: the language's own spelling,
// brackets and all.
//
// It is not a C identifier — no C program can define a symbol of this name,
// which is exactly the point. A crash report reads `-[NSString length]`
// because that is what Apple's tools name the function.
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

// MethodSymbol is what a method's function is *named* in the IR and in the
// object file.
//
// It cannot be MethodName. A VIR symbol is an identifier — letters, digits,
// underscore and dollar — so the bracketed spelling is not representable,
// and neither is it representable in an ELF or COFF symbol table without
// quoting that assemblers disagree about.
//
// The mangling is libobjc2's, not an invention: the GNU runtime has always
// named a method's function `_i_Class__selector` for an instance method and
// `_c_Class__selector` for a class method, with the category between the two
// underscores and every colon of the selector written as an underscore. It
// is unambiguous — a selector's colons are recoverable, since an identifier
// cannot contain one — and it is what a debugger on a GNUstep system already
// knows how to read.
//
//	-[NSString length]              _i_NSString__length
//	+[Cache cacheWithCapacity:]     _c_Cache__cacheWithCapacity_
//	-[Cache(Extra) extra]           _i_Cache_Extra_extra
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

	// MsgSendSuper2 is the super send. The 2 is not a version: it takes the
	// *class* rather than its superclass and looks the superclass up itself,
	// which is what lets a category on a superclass be attached after the
	// subclass was compiled.
	MsgSendSuper2      = "objc_msgSendSuper2"
	MsgSendSuper2Stret = "objc_msgSendSuper2_stret"

	// The ARC entry points. Each is a call lower emits where ownership
	// changes; none of them is a message send, and that is the point — they
	// are functions the runtime exports, so a retain is a call and not a
	// dispatch.
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

	// CxxDestructSelector is the method objc4 looks up on a class as it
	// deallocates an object, and calls if it is there. C++ named the hook
	// and Objective-C borrowed it, because the question is the same one: an
	// object is going away and its members have to be let go. No program
	// can write the name, which is what makes it safe to own.
	CxxDestructSelector = ".cxx_destruct"

	AutoreleasePoolPush = "objc_autoreleasePoolPush"
	AutoreleasePoolPop  = "objc_autoreleasePoolPop"

	// GetProperty is what a synthesized getter calls when the property is
	// atomic: reading a pointer and retaining it have to happen without a
	// setter running in between, and the runtime owns the lock that makes
	// that true.
	//
	//	id objc_getProperty(id self, SEL _cmd, ptrdiff_t offset, BOOL atomic);
	GetProperty = "objc_getProperty"

	// RetainBlock is what retaining a *block* is, and the difference is not
	// a detail: a block literal is a stack object, and retaining it has to
	// copy it to the heap first or the reference outlives the frame. Every
	// other object is retained with Retain.
	//
	//	id objc_retainBlock(id);
	RetainBlock = "objc_retainBlock"

	// Exceptions (§7.2) and synchronization (§7.3).
	ExceptionThrow   = "objc_exception_throw"
	ExceptionRethrow = "objc_exception_rethrow"
	SyncEnter        = "objc_sync_enter"
	SyncExit         = "objc_sync_exit"

	// Personality is the routine the unwinder runs for a frame with a
	// @try in it. It reads the same Itanium-ABI tables a C++ frame's
	// personality does; what makes it Objective-C's is how it compares a
	// thrown object against the type-info a @catch names.
	Personality = "__objc_personality_v0"

	// BeginCatch hands a @catch its object and makes it the exception
	// being handled; EndCatch says the clause is done with it.
	//
	//	id objc_begin_catch(void *exn);
	//	void objc_end_catch(void);
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
