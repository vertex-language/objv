package sysroot

import (
	"embed"
	"io/fs"
)

// The builtin headers are compiler property, not libc property.
//
// <stdarg.h>, <stddef.h>, <stdint.h> and kin are the headers a hosted libc
// assumes the compiler ships, and the complete set a freestanding
// implementation requires (§4p6). Apple's SDK ships none of them — clang's
// own usr/lib/clang/<v>/include is the first directory on every Darwin
// include list, and this is objv's answer to it.
//
// They are written once, against target-parameterized predefined macros, so
// one text serves every target. The contract: the objv package computes the
// following from types.Model and passes them through
// preprocessor.Config.Predefines —
//
//	__CHAR_BIT__      __CHAR_UNSIGNED__ (defined iff char is unsigned)
//	__SCHAR_MAX__     __SHRT_MAX__      __INT_MAX__
//	__LONG_MAX__      __LONG_LONG_MAX__
//	__SIZEOF_SHORT__  __SIZEOF_INT__    __SIZEOF_LONG__
//	__SIZEOF_LONG_LONG__                __SIZEOF_POINTER__
//	__SIZE_TYPE__     __SIZE_MAX__
//	__PTRDIFF_TYPE__  __PTRDIFF_MAX__
//	__WCHAR_TYPE__    __WCHAR_MAX__     __WCHAR_MIN__
//	__WINT_TYPE__     __WINT_MAX__      __WINT_MIN__
//	__INTMAX_TYPE__   __UINTMAX_TYPE__  __INTMAX_MAX__
//	and float.h's __FLT_*__/__DBL_*__/__LDBL_*__ set.
//
// The names are gcc's spellings, on purpose: Apple's headers already test
// several of them, and inventing a second vocabulary for the same facts would
// buy nothing.
//
// Nothing Objective-C is here. `id`, `Class`, `SEL`, `BOOL`, `nil` and `YES`
// come from <objc/objc.h>, which belongs to the runtime and is shipped by the
// SDK — clang carries no Objective-C header either, and a compiler that
// shipped its own copy would be declaring the runtime's types on the
// runtime's behalf.
//
//go:embed builtin/*.h
var builtinFS embed.FS

// builtinEntry mounts the embedded headers. The name is <builtin>: never
// absolute, so __FILE__ and diagnostics read "<builtin>/stdarg.h" on every
// machine identically.
func builtinEntry() Entry {
	sub, err := fs.Sub(builtinFS, "builtin")
	if err != nil {
		// The directory is compiled into the binary; its absence is a
		// build error of objv itself, not a runtime condition.
		panic("sysroot: embedded builtin headers missing: " + err.Error())
	}
	return Entry{Name: "<builtin>", FS: sub, System: true}
}

// BuiltinFS is the embedded header set, for a caller that wants to list or
// print it — `objv env` does.
func BuiltinFS() fs.FS {
	sub, _ := fs.Sub(builtinFS, "builtin")
	return sub
}
