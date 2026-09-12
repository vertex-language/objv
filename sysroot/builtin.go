package sysroot

import (
	"embed"
	"io/fs"
)

// Builtin headers (<stdarg.h>, <stddef.h>, <stdint.h>, etc.) are compiler-provided
// freestanding headers parameterized by predefined macros from types.Model.
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
