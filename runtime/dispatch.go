package runtime

import "github.com/vertex-language/objv/types"

// Which objc_msgSend a send compiles to.
//
// There is more than one because the calling convention has more than one
// shape for a return value, and the runtime's trampoline has to know which
// one it is forwarding: a function that returns a struct by hidden pointer
// takes an extra argument, and one that returns a float on the x87 stack
// leaves it somewhere the ordinary path would clobber. The variants are not
// optimizations — calling the wrong one corrupts the return value.
//
// Which variant a return type needs is the *target's* question, not the
// language's, so it is asked of an Arch rather than answered once.

// Arch is the target architecture, as far as dispatch is concerned.
type Arch uint8

const (
	ARM64 Arch = iota
	AMD64
	I386
)

// Send names the entry point for one message send.
//
// super says the send is `[super …]`, which goes through a different
// trampoline: objc_msgSendSuper2 takes the class the method was compiled in
// and finds the superclass itself, so that a category attached to the
// superclass afterwards is still found.
func (a ABI) Send(arch Arch, ret types.Type, m types.Model, super bool) string {
	switch a.stret(arch, ret, m) {
	case true:
		if super {
			return MsgSendSuper2Stret
		}
		return MsgSendStret
	}
	if !super {
		if name, ok := a.fpret(arch, ret); ok {
			return name
		}
	}
	if super {
		return MsgSendSuper2
	}
	return MsgSend
}

// stret reports whether the return value is passed by hidden pointer, which
// is what makes a send objc_msgSend_stret.
//
// arm64 has no stret variant at all: the indirect result register is
// separate from the argument registers, so the ordinary trampoline forwards
// it without knowing it is there. On x86-64 a struct larger than sixteen
// bytes, or one the classifier sends to memory, needs the hidden pointer and
// therefore the other entry point. On i386 every struct return does.
func (a ABI) stret(arch Arch, ret types.Type, m types.Model) bool {
	if arch == ARM64 || ret == nil {
		return false
	}
	u := types.Unqualify(ret)
	if u.Kind() != types.StructKind && u.Kind() != types.UnionKind {
		return false
	}
	if arch == I386 {
		return true
	}
	size, ok := m.Sizeof(u)
	return !ok || size > 16
}

// fpret reports whether the return value goes through one of the
// floating-point trampolines.
//
// It is an x86 story. On i386 every floating return is on the x87 stack and
// needs objc_msgSend_fpret; on x86-64 only long double is, and _Complex long
// double needs the two-register variant. arm64 returns floats in the
// ordinary registers and has neither entry point.
func (a ABI) fpret(arch Arch, ret types.Type) (string, bool) {
	if ret == nil || arch == ARM64 {
		return "", false
	}
	switch types.Unqualify(ret).Kind() {
	case types.LongDouble:
		return MsgSendFpret, true
	case types.ComplexLongDouble:
		if arch == AMD64 {
			return MsgSendFp2ret, true
		}
		return MsgSendFpret, true
	case types.Float, types.Double, types.ComplexFloat, types.ComplexDouble:
		if arch == I386 {
			return MsgSendFpret, true
		}
	}
	return "", false
}
