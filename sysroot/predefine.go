package sysroot

import "strconv"

// The platform's predefined macros.
//
// These are the ones that are facts about *this platform, this SDK and this
// deployment target* — not about the type model, which is types.Model's to
// state, and not about the language, which is the front end's. The split
// matters because the platform's spellings are the platform's: __LITTLE_ENDIAN__
// is an architecture fact written the way Darwin writes it, and nowhere else
// writes it that way at all.
//
// Every one of them is load-bearing in a real build, and each was found by
// preprocessing <Foundation/Foundation.h> and reading what broke:
//
//	__LITTLE_ENDIAN__   CFBase.h and NSByteOrder.h #error without it
//	__APPLE_CC__        TargetConditionals.h's compiler test, beside __GNUC__
//	__ENVIRONMENT_...   what every availability macro in every Cocoa header
//	                    compares against, and therefore what decides whether
//	                    a method is declared at all
//
// The result is spellings, one per macro, in the form -D takes: they go
// through preprocessor.Config.Predefines and are parsed by the same #define
// grammar a directive is, so a predefine and a #define cannot drift apart.
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

		// clang's value, which is what TargetConditionals.h tests for
		// beside __GNUC__ and what several headers compare against. It
		// stopped tracking any real Apple compiler version long ago.
		"__APPLE_CC__=6000",

		// Mach-O is position-independent and dynamically linked by
		// default, and a few headers key on both.
		"__DYNAMIC__=1",
		"__PIC__=2",
		"__pic__=2",
	}
	out = append(out, endianMacros(archOf(opt.Target))...)
	out = append(out, archMacros(archOf(opt.Target))...)

	if !r.Deployment.IsZero() {
		// The macro every availability check in every Cocoa header reads.
		// Availability.h turns it into __OSX_AVAILABLE and its family, and
		// a header whose method is newer than this simply does not declare
		// it — which is what makes a deployment target a compile-time
		// thing rather than a note in the Info.plist.
		//
		// Both spellings, because AvailabilityInternal.h chooses between
		// them by asking __has_builtin(__is_target_os): a compiler that has
		// that builtin is expected to publish the platform-neutral name and
		// is never asked for the other. objv has the builtin and published
		// only the other, so __MAC_OS_X_VERSION_MIN_REQUIRED came out as an
		// undefined identifier — which in a #if is zero, so every
		// availability comparison in the SDK silently took the branch for
		// an OS older than anything, and headers declared the wrong things.
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
		return []string{"__aarch64__=1", "__arm64__=1", "__arm64=1", "__LP64__=1", "_LP64=1"}
	case "x86_64":
		return []string{"__x86_64__=1", "__x86_64=1", "__amd64__=1", "__amd64=1",
			"__LP64__=1", "_LP64=1"}
	case "i386":
		return []string{"__i386__=1", "__i386=1"}
	}
	return nil
}
