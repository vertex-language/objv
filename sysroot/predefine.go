package sysroot

import "strconv"

// Predefines returns platform, SDK, and architecture predefined macros in -D format.
func Predefines(opt Options, r Result) []string {
	switch osOf(opt.Target) {
	case "macos":
		return darwinPredefines(opt, r)
	case "linux":
		return gnustepPredefines(opt, "__linux__")
	case "windows":
		return gnustepPredefines(opt, "_WIN32")
	}
	return nil
}

func darwinPredefines(opt Options, r Result) []string {
	out := []string{
		"__APPLE__=1",
		"__MACH__=1",
		"__APPLE_CC__=6000",
		"__DYNAMIC__=1",
		"__PIC__=2",
		"__pic__=2",
	}
	out = append(out, endianMacros(archOf(opt.Target))...)
	out = append(out, archMacros(archOf(opt.Target))...)

	if !r.Deployment.IsZero() {
		// Publish both platform-neutral and legacy macOS deployment target macros.
		v := strconv.Itoa(r.Deployment.MacroValue())
		out = append(out,
			"__ENVIRONMENT_MAC_OS_X_VERSION_MIN_REQUIRED__="+v,
			"__ENVIRONMENT_OS_VERSION_MIN_REQUIRED__="+v)
	}
	return out
}

// gnustepPredefines is the non-Darwin platform's much shorter list. There is
// no SDK to version and no availability table to gate on: a GNUstep
// installation declares what it has, and a program that wants something older
// links against an older one.
func gnustepPredefines(opt Options, osMacro string) []string {
	out := []string{osMacro + "=1", "__unix__=1"}
	if osMacro == "_WIN32" {
		out = []string{osMacro + "=1"}
	}
	out = append(out, endianMacros(archOf(opt.Target))...)
	out = append(out, archMacros(archOf(opt.Target))...)
	return out
}

// endianMacros are both spellings: gcc's __BYTE_ORDER__ against the three
// __ORDER_*__ constants, which portable code uses, and Darwin's bare
// __LITTLE_ENDIAN__, which CoreFoundation uses and which is not defined at
// all on the other side.
//
// Every architecture objv targets is little-endian. The big-endian branch is
// written anyway, because a table with one row is a table that gets read as a
// constant and then quietly assumed.
func endianMacros(arch string) []string {
	out := []string{
		"__ORDER_LITTLE_ENDIAN__=1234",
		"__ORDER_BIG_ENDIAN__=4321",
		"__ORDER_PDP_ENDIAN__=3412",
	}
	if bigEndian[arch] {
		return append(out, "__BYTE_ORDER__=__ORDER_BIG_ENDIAN__", "__BIG_ENDIAN__=1")
	}
	return append(out, "__BYTE_ORDER__=__ORDER_LITTLE_ENDIAN__", "__LITTLE_ENDIAN__=1")
}

var bigEndian = map[string]bool{}

// archMacros are the names a header tests to learn the architecture. Each
// architecture answers to several, because different headers picked
// different ones and all of them are still in use.
func archMacros(arch string) []string {
	switch arch {
	case "aarch64":
		// __aarch64__ is the architecture's own name and __arm64__ is
		// Apple's; CarbonCore's fp.h #errors on a CPU it does not
		// recognize, and recognizes the second.
		//
		// __ARM_NEON says the vector unit is there, which on AArch64 it
		// always is — the architecture has no variant without it. What a
		// header does with the answer is include <arm_neon.h>, which is
		// the compiler's to supply and which objv supplies: see
		// builtin/arm_neon.h. Saying yes without it would leave
		// <simd/simd.h> calling names nothing declared.
		return []string{"__aarch64__=1", "__arm64__=1", "__arm64=1",
			"__ARM_NEON=1", "__ARM_NEON__=1", "__ARM_FP=0xE",
			"__LP64__=1", "_LP64=1"}
	case "x86_64":
		return []string{"__x86_64__=1", "__x86_64=1", "__amd64__=1", "__amd64=1",
			"__LP64__=1", "_LP64=1"}
	case "i386":
		return []string{"__i386__=1", "__i386=1"}
	}
	return nil
}
