package runtime

// Where the metadata goes.
//
// The section names are the runtime's interface as much as the structures
// are: objc4 finds a class not by looking for a symbol but by walking
// __objc_classlist in every loaded image, which is why every list below is
// marked no_dead_strip. A class nothing references is still a class the
// runtime must register, because a message may name it at run time.
//
// A Mach-O section name here is the whole attribute line the assembler takes,
// segment included: the segment decides whether the data is written to
// (__DATA) or shared read-only (__TEXT), and the attributes decide what the
// linker may do with it.

// Section names a piece of metadata's home.
type Section uint8

const (
	// The lists the runtime walks at load.
	SecClassList Section = iota
	SecCategoryList

	// The non-lazy halves of the two above. A class or category that
	// implements +load goes in both: the ordinary list is what the runtime
	// realizes on demand, and this one is what it walks at image load to
	// find the +loads to call. A class that is only in the first has its
	// +load found by nobody.
	SecNonLazyClassList
	SecNonLazyCategoryList
	SecProtocolList
	SecImageInfo

	// The metadata itself. __objc_data holds what the runtime writes to —
	// a class object's isa and cache — and __objc_const holds what it only
	// reads.
	SecClassData
	SecConst
	SecIvarOffsets
	SecProtocolData

	// The references the runtime rewrites at load.
	SecSelectorRefs
	SecClassRefs
	SecSuperRefs

	// The strings. They live in __TEXT because nothing writes to them, and
	// in separate sections because the linker merges each kind against the
	// other images'.
	SecClassNames
	SecMethodNames
	SecMethodTypes

	// Where a @"…" literal's object and its characters go. The object is
	// writable because the runtime may rewrite the isa; the characters are
	// not, and their two sections differ only in the width of a code unit.
	SecCFString
	SecCString
	SecUString

	// A block's descriptor, and the literal of a block that captured
	// nothing. Both are constant and both hold pointers, so they go where
	// clang puts them: (__DATA,__const), which the linker moves into
	// __DATA_CONST when it builds the image. Not __TEXT — a pointer there
	// could not be relocated.
	SecBlockConst
)

// Name is the section a piece of metadata goes in, spelled the way this
// ABI's container spells it.
func (a ABI) Name(s Section) string {
	if a.Container != MachO {
		return elfSections[s]
	}
	return machoSections[s]
}

// Mach-O, as clang emits them. The attributes are load-bearing:
//
//   - no_dead_strip on every list, because the runtime finds its work by
//     walking them and the linker cannot see that;
//   - coalesced on the protocol list, because every image that mentions a
//     protocol defines it and exactly one copy must survive;
//   - literal_pointers on the selector references, which lets the linker
//     merge two references to the same selector;
//   - cstring_literals on the string sections, which merges equal strings
//     across the whole link.
var machoSections = [...]string{
	SecClassList:    "__DATA,__objc_classlist,regular,no_dead_strip",
	SecCategoryList: "__DATA,__objc_catlist,regular,no_dead_strip",

	SecNonLazyClassList:    "__DATA,__objc_nlclslist,regular,no_dead_strip",
	SecNonLazyCategoryList: "__DATA,__objc_nlcatlist,regular,no_dead_strip",
	SecProtocolList:        "__DATA,__objc_protolist,coalesced,no_dead_strip",
	SecImageInfo:           "__DATA,__objc_imageinfo,regular,no_dead_strip",

	SecClassData:   "__DATA,__objc_data",
	SecConst:       "__DATA,__objc_const",
	SecIvarOffsets: "__DATA,__objc_ivar",
	// A protocol object is ordinary writable data, so it goes where
	// ordinary writable data goes and names no section of its own. Naming
	// it "__DATA,__data" would be the same section under a second name,
	// which a Mach-O object cannot hold: an object with a protocol and any
	// other mutable global would ask for __DATA,__data twice.
	SecProtocolData: "",

	SecSelectorRefs: "__DATA,__objc_selrefs,literal_pointers,no_dead_strip",
	SecClassRefs:    "__DATA,__objc_classrefs,regular,no_dead_strip",
	SecSuperRefs:    "__DATA,__objc_superrefs,regular,no_dead_strip",

	SecClassNames:  "__TEXT,__objc_classname,cstring_literals",
	SecMethodNames: "__TEXT,__objc_methname,cstring_literals",
	SecMethodTypes: "__TEXT,__objc_methtype,cstring_literals",

	SecCFString: "__DATA,__cfstring",
	SecCString:  "__TEXT,__cstring,cstring_literals",
	SecUString:  "__TEXT,__ustring",

	SecBlockConst: "__DATA,__const",
}

// ELF and COFF have no segments and no section attributes, so the names are
// the bare ones libobjc2 looks for. The lists still have to survive dead
// stripping; on ELF that is the linker's --gc-sections and the SHF_GNU_RETAIN
// flag the object writer sets, not something the name can say.
var elfSections = [...]string{
	SecClassList:    "__objc_classlist",
	SecCategoryList: "__objc_catlist",

	SecNonLazyClassList:    "__objc_nlclslist",
	SecNonLazyCategoryList: "__objc_nlcatlist",
	SecProtocolList:        "__objc_protolist",
	SecImageInfo:           "__objc_imageinfo",

	SecClassData:    "__objc_data",
	SecConst:        "__objc_const",
	SecIvarOffsets:  "__objc_ivar",
	SecProtocolData: "",

	SecSelectorRefs: "__objc_selrefs",
	SecClassRefs:    "__objc_classrefs",
	SecSuperRefs:    "__objc_superrefs",

	SecClassNames:  "__objc_classname",
	SecMethodNames: "__objc_methname",
	SecMethodTypes: "__objc_methtype",

	// libobjc2's constant string is an NSConstantString and not a
	// CFString, so it is an ordinary object in an ordinary data section.
	SecCFString: ".data",
	SecCString:  ".rodata",
	SecUString:  ".rodata",

	SecBlockConst: ".data.rel.ro",
}
