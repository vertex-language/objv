package objv

// The linkers dispatch on a target through a registry each architecture
// package fills in from its own init. Importing them for effect is what
// registers them, and without it every link fails with "no backend
// registered" — which is a link that resolved every symbol and then had
// nowhere to write them.
//
// They are here rather than in link.go so that the reason is stated once,
// beside the list, instead of looking like an unused import somebody forgot
// to remove.
import (
	_ "github.com/vertex-language/macho/arm64"
	_ "github.com/vertex-language/macho/x86_64"

	_ "github.com/vertex-language/elf/arm64"
	_ "github.com/vertex-language/elf/x86_64"

	_ "github.com/vertex-language/pe/x64"
)
