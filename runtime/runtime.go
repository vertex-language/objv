// Package runtime describes the Objective-C runtime ABI: the symbols a
// translation unit defines and references, the sections its metadata is
// placed in, the layout of the structures the runtime walks, and the type
// encodings it reads out of them.
//
// It is data and rules, not emission. Nothing here writes an object file or
// builds IR — lower does that, and it does it by asking this package what
// each thing is called, how big it is, and what goes in it. The split is
// deliberate: the ABI is a contract with a runtime nobody here controls, and
// a contract stated in one place can be read, tested and corrected without
// touching a code generator.
//
// The layouts are the modern (non-fragile) runtime's, which is the only one
// objv emits for. Under it a class's instance variables have no offsets until
// the program loads — the runtime assigns them, through the offset variables
// this package names — which is what lets a framework add an instance
// variable without every program that subclasses it having to be rebuilt.
package runtime

// Kind is which runtime a target uses.
//
// The two differ in more than a name. Apple's is the one on every Darwin
// platform, and its metadata is what objc4 walks; GNUstep's libobjc2 is what
// Linux and Windows have, and it reads a different set of structures. objv
// emits Apple's today; the GNUstep layouts are named where they differ so
// that adding them is filling in a table rather than finding the seams.
type Kind uint8

const (
	// AppleModern is objc4's non-fragile ABI, version 2.
	AppleModern Kind = iota
	// GNUstep is libobjc2's ABI.
	GNUstep
)

func (k Kind) String() string {
	if k == GNUstep {
		return "gnustep"
	}
	return "apple"
}

// Container is the object file format, which decides how sections are
// named.
type Container uint8

const (
	MachO Container = iota
	ELF
	COFF
)

// ABI is the runtime ABI of one target: which runtime, which container, and
// how wide a pointer is.
//
// A zero ABI is Apple's on Mach-O with 64-bit pointers, which is the target
// objv is built for first and the one every layout here was checked against.
type ABI struct {
	Kind      Kind
	Container Container
	PtrBytes  int64
}

// Darwin64 is the ABI of aarch64-macos and x86_64-macos.
func Darwin64() ABI {
	return ABI{Kind: AppleModern, Container: MachO, PtrBytes: 8}
}

// ptr is the pointer width, defaulting to 64-bit.
func (a ABI) ptr() int64 {
	if a.PtrBytes == 0 {
		return 8
	}
	return a.PtrBytes
}
