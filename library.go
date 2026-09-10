package objv

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// -l, -L, -framework and -F: how a name becomes a file.
//
// The vertex-language linkers take bytes, and only the PE one has a search
// path of its own. So resolving a name is objv's job — which is the right
// place for it anyway, because the spelling a name resolves through is a fact
// of the target's platform and objv is what knows the target.
//
// The rule is the one every C toolchain has: the caller's directories in the
// order they were given, then the platform's, and within each directory the
// shared form before the static one unless the link is static. A name that
// resolves in an earlier directory wins; a name that resolves nowhere is an
// error naming what was looked for and where, because "cannot find -lobjc"
// without the search list is a message that sends the reader to strace.

// libraryNames is the filenames "-l name" can mean on this target, in the
// order they are tried.
//
// Mach-O looks for a .tbd first because that is what a modern macOS SDK ships
// and what the linker can actually read: the dylib itself lives in the shared
// cache, and a .dylib on disk needs an exports reader that is not written.
func libraryNames(t Target, name string, static bool) []string {
	archive := "lib" + name + ".a"
	if t.format == FormatPE {
		// MSVC's convention is the reverse of Unix's: foo.lib is the import
		// library that binds to foo.dll, and libfoo.lib is the static one.
		if static {
			return []string{"lib" + name + ".lib", archive, name + ".lib"}
		}
		return []string{name + ".lib", "lib" + name + ".lib", archive}
	}
	if static {
		return []string{archive}
	}
	switch t.format {
	case FormatMachO:
		return []string{"lib" + name + ".tbd", "lib" + name + ".dylib", archive}
	case FormatELF:
		return []string{"lib" + name + ".so", archive}
	}
	return []string{archive}
}

// findLibrary resolves one -l name against the search list and reads it.
func findLibrary(t Target, name string, dirs []string, static bool) (Input, error) {
	names := libraryNames(t, name, static)
	for _, dir := range dirs {
		for _, base := range names {
			path := filepath.Join(dir, base)
			data, err := os.ReadFile(path)
			switch {
			case err == nil:
				return Input{Name: path, Data: data}, nil
			case os.IsNotExist(err):
				continue
			default:
				// Found and unreadable is not "keep looking": a directory
				// the caller named holds a library it cannot open, and
				// searching past it would report the wrong problem.
				return Input{}, fmt.Errorf("%s: %w", path, err)
			}
		}
	}
	return Input{}, fmt.Errorf("cannot find -l%s: no %s in %s",
		name, orList(names), dirList(dirs))
}

// findFramework finds one framework's link input.
//
// A framework is a directory with the library inside it under the same name,
// so the search is by shape rather than by filename: -framework Foundation
// looks for Foundation.framework/Foundation.tbd and then the binary beside
// it. That is why -l cannot spell one — libFoundation.tbd does not exist and
// never did — and why an Objective-C driver needs this and a C one does not.
func findFramework(name string, dirs []string) (Input, error) {
	bases := []string{name + ".tbd", name}
	for _, dir := range dirs {
		for _, base := range bases {
			path := filepath.Join(dir, name+".framework", base)
			data, err := os.ReadFile(path)
			switch {
			case err == nil:
				return Input{Name: path, Data: data}, nil
			case os.IsNotExist(err):
				continue
			default:
				return Input{}, fmt.Errorf("%s: %w", path, err)
			}
		}
	}
	return Input{}, fmt.Errorf("cannot find -framework %s: no %s.framework in %s",
		name, name, dirList(dirs))
}

func orList(names []string) string {
	switch len(names) {
	case 0:
		return "nothing"
	case 1:
		return names[0]
	case 2:
		return names[0] + " or " + names[1]
	}
	return strings.Join(names[:len(names)-1], ", ") + " or " + names[len(names)-1]
}

func dirList(dirs []string) string {
	if len(dirs) == 0 {
		return "no directories (name one with -L or -F)"
	}
	return strings.Join(dirs, ", ")
}
