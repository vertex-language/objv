package runtime

import "strings"

// The flag words, and the property attribute string.

// ClassFlag is a bit of class_ro_t.flags.
type ClassFlag uint32

const (
	// ROMeta marks the metaclass. It is what tells the runtime that the
	// methods in this class_ro_t are the class methods.
	ROMeta ClassFlag = 1 << 0
	// RORoot marks a class with no superclass. The runtime closes the
	// metaclass chain here: a root class's metaclass's isa is the root
	// metaclass itself.
	RORoot ClassFlag = 1 << 1
	// ROHasCXXStructors says the class has a .cxx_construct or a
	// .cxx_destruct — which in Objective-C means it has instance variables
	// the compiler must clean up: a __strong ivar is released and a __weak
	// one is unregistered, and .cxx_destruct is where that happens.
	ROHasCXXStructors ClassFlag = 1 << 2
	ROHidden          ClassFlag = 1 << 4
	// ROException marks a class usable as an exception type.
	ROException ClassFlag = 1 << 5
	// ROIsARC says the class was compiled with ARC. The runtime reads it to
	// know that the ivars are managed.
	ROIsARC ClassFlag = 1 << 7
	// ROHasCXXDtorOnly narrows ROHasCXXStructors: there is a destructor and
	// no constructor, which is the ordinary case for a class whose only
	// cleanup is releasing its ivars.
	ROHasCXXDtorOnly ClassFlag = 1 << 8
	// ROHasWeakWithoutARC marks a class with __weak ivars compiled without
	// ARC, which the runtime has to treat differently because nothing else
	// will clear them.
	ROHasWeakWithoutARC ClassFlag = 1 << 9
)

// ImageFlag is a bit of the image info word.
type ImageFlag uint32

const (
	// ImageIsReplacement and ImageSupportsGC are historical: garbage
	// collection is gone from every runtime objv targets, and the bits are
	// named so that a reader of a dumped image knows what the zeros mean.
	ImageIsReplacement       ImageFlag = 1 << 0
	ImageSupportsGC          ImageFlag = 1 << 1
	ImageRequiresGC          ImageFlag = 1 << 2
	ImageOptimizedByDyld     ImageFlag = 1 << 3
	ImageCorrectedSynthesize ImageFlag = 1 << 4
	ImageIsSimulated         ImageFlag = 1 << 5
	// ImageHasCategoryClassProperties says the category structures in this
	// image carry the classProperties field. Every image a current compiler
	// emits sets it, because every category_t it writes has the field —
	// which is what makes the flag a statement about the layout rather than
	// about the program.
	ImageHasCategoryClassProperties ImageFlag = 1 << 6
)

// ImageInfoFlags is what an image compiled by objv carries.
//
// The category class properties bit is set because the category_t this
// package describes has the field. Nothing else is: garbage collection is
// gone, dyld's optimizations are dyld's to claim, and a simulator image is
// one built against a simulator SDK, which is a fact about the SDK and not
// about the compiler.
func ImageInfoFlags() ImageFlag { return ImageHasCategoryClassProperties }

// ClassFlags is the flag word for a class or its metaclass.
func ClassFlags(meta, root, arc, hasDestructor, hasWeakWithoutARC bool) ClassFlag {
	var f ClassFlag
	if meta {
		f |= ROMeta
	}
	if root {
		f |= RORoot
	}
	if hasDestructor {
		// The two go together: objc4 checks ROHasCXXStructors to know
		// whether to look for the methods, and ROHasCXXDtorOnly to know it
		// need not look for a constructor.
		f |= ROHasCXXStructors | ROHasCXXDtorOnly
	}
	if arc {
		f |= ROIsARC
	}
	if hasWeakWithoutARC {
		f |= ROHasWeakWithoutARC
	}
	return f
}

// ---- property attributes ----

// PropertyDesc is what a property attribute string is built from. It is the
// property as the runtime needs to see it, which is not quite as the language
// declares it: what matters here is the encoding, the ownership, and the
// names of the three things a program can reach it through.
type PropertyDesc struct {
	// Type is the extended encoding of the property's type: @"NSString"
	// rather than @, because a program reading the attributes back wants to
	// know the class.
	Type string

	Readonly  bool
	Copy      bool
	Retain    bool // strong or retain: the runtime knows one behaviour
	Weak      bool
	Nonatomic bool
	Dynamic   bool

	// Getter and Setter are the custom selectors, empty where the property
	// uses the default ones. Ivar is the backing variable, empty for a
	// @dynamic property and for a readonly one the class implements by hand.
	Getter string
	Setter string
	Ivar   string
}

// PropertyAttributes builds the attribute string.
//
//	@property (nonatomic, copy) NSString *name;   →  T@"NSString",C,N,V_name
//	@property (readonly, getter=isOn) int on;     →  Ti,R,GisOn
//
// The order is the runtime's, and it is not alphabetical or grouped by
// meaning: type, readonly, ownership, dynamic, atomicity, getter, setter,
// backing variable. A reader that parses the string does so left to right
// and stops caring after the type, so the order is what a *writer* has to
// match rather than something anything depends on — which is exactly why it
// is stated once, here, instead of at the place that assembles it.
func PropertyAttributes(p PropertyDesc) string {
	var parts []string
	parts = append(parts, "T"+p.Type)
	if p.Readonly {
		parts = append(parts, "R")
	}
	switch {
	case p.Copy:
		parts = append(parts, "C")
	case p.Retain:
		parts = append(parts, "&")
	case p.Weak:
		parts = append(parts, "W")
		// assign and unsafe_unretained say nothing: the absence of an
		// ownership letter is what they are.
	}
	if p.Dynamic {
		parts = append(parts, "D")
	}
	if p.Nonatomic {
		parts = append(parts, "N")
	}
	if p.Getter != "" {
		parts = append(parts, "G"+p.Getter)
	}
	if p.Setter != "" {
		parts = append(parts, "S"+p.Setter)
	}
	if p.Ivar != "" {
		parts = append(parts, "V"+p.Ivar)
	}
	return strings.Join(parts, ",")
}
