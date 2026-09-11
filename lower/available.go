package lower

import (
	"github.com/vertex-language/ir"

	"github.com/vertex-language/objv/ast"
	"github.com/vertex-language/objv/runtime"
)

// §6.10's @available.
//
// The question is about the machine the program is *running* on. An image
// with a deployment target of macOS 11 may be launched on 12, and this is
// how it finds out — which is why a framework API introduced later than the
// deployment target can be called at all.
//
// Two of the three answers are constants. A clause naming this platform that
// the deployment target already satisfies is true and nothing is emitted for
// it: the linker recorded the deployment target in LC_BUILD_VERSION, so the
// comparison was settled before the program ran. A list naming no platform
// this image is for is true as well — that is what the trailing `*` means,
// and it is what lets one source file carry checks for platforms it is not
// being built for. Only a clause about this platform asking for something
// newer than the deployment target becomes a call.
//
// runtime/availability.go says which call and why it is not clang's.

func (u *unit) availability(e *ast.AvailabilityExpr) ir.Value {
	b := u.fn.cur
	want, named := u.availabilityWanted(e)
	if !named || u.deployment.AtLeast(want) {
		return b.I32.Const(1)
	}

	// A dyld_build_version_t in the frame, and the address of it: the check
	// takes an array, and one element is an array of one.
	slot := u.fn.entry.Ptr.Alloc(uint64(u.abi.SizeOf(runtime.BuildVersion)), 4)
	u.fn.entry.Name(slot, "avail")
	b.I32.Store(b.I32.Const(int64(u.platform)), slot)
	b.I32.Store(b.I32.Const(int64(want.Encoded())),
		b.Ptr.Add(slot, b.I64.Const(4)))

	sig := ir.NewSig()
	sig.Param(ir.TypeI32)
	sig.Param(ir.TypePtr)
	sig.Ret(ir.TypeI32)
	res := b.Call(u.extern(runtime.AvailabilityCheck, sig), b.I32.Const(1), slot)
	if res.Len() == 0 {
		return nil
	}
	v, ok := res.Value(0).(ir.I32)
	if !ok {
		return nil
	}
	// The check returns a C bool, whose value is in the low bit and whose
	// upper bits are not promised. §6.10's expression is an int that is 0
	// or 1, so the rest is masked off rather than trusted.
	return b.I32.And(v, b.I32.Const(1))
}

// availabilityWanted is the version this list asks for on *this* platform,
// and whether it names it at all.
func (u *unit) availabilityWanted(e *ast.AvailabilityExpr) (runtime.OSVersion, bool) {
	for _, sp := range e.Specs {
		if sp == nil || sp.Platform == nil {
			continue
		}
		p, ok := runtime.PlatformNamed(u.name(sp.Platform))
		if !ok || p != u.platform {
			continue
		}
		v, ok := runtime.ParseOSVersion(string(u.src.Slice(sp.Version.Lo, sp.Version.Hi)))
		if !ok {
			continue
		}
		return v, true
	}
	return runtime.OSVersion{}, false
}
