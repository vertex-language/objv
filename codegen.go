package objv

import (
	"bytes"
	"fmt"

	amd64elf "github.com/vertex-language/amd64/obj/elf"
	amd64macho "github.com/vertex-language/amd64/obj/macho"
	amd64pe "github.com/vertex-language/amd64/obj/pe"
	arm64elf "github.com/vertex-language/arm64/obj/elf"
	arm64macho "github.com/vertex-language/arm64/obj/macho"
	machocore "github.com/vertex-language/macho"

	"github.com/vertex-language/ir"
	amd64lower "github.com/vertex-language/ir/lower/amd64"
	arm64lower "github.com/vertex-language/ir/lower/arm64"
)

// emitObject lowers m for t and returns the object file's bytes.
//
// This is the pipeline below VIR: ir/lower selects instructions for one
// architecture, and that architecture's obj/{elf,macho,pe} writes them into a
// container. The two halves are chosen independently — Arch picks the
// backend, Format picks the writer — and a pair that does not exist is an
// error naming both rather than a nil object.
//
// Nothing about Objective-C reaches here. By this point a class is a few
// globals in named sections and a method is a function, which is the whole
// point of stopping the language at VIR: the backend never learns what a
// selector is.
func emitObject(m *ir.Module, t Target, producer string, dep string) ([]byte, error) {
	if m == nil {
		return nil, fmt.Errorf("nil module")
	}
	if err := t.Supports(); err != nil {
		return nil, err
	}
	switch t.arch {
	case ArchARM64:
		return arm64Object(m, t, producer, dep)
	case ArchAMD64:
		return amd64Object(m, t, producer, dep)
	}
	return nil, fmt.Errorf("target %s names no backend", t.name)
}

func arm64Object(m *ir.Module, t Target, producer, dep string) ([]byte, error) {
	// Apple's variadic convention is a fact of the platform, and the
	// platform is what the container says: both Darwin and Linux declare
	// abi "aapcs" in the layout block, so this cannot be read off the
	// module. Getting it wrong is a wrong call, not a slow one.
	variadic := arm64lower.VariadicAAPCS64
	if t.format == FormatMachO {
		variadic = arm64lower.VariadicDarwin
	}
	o, err := arm64lower.Lower(m, arm64lower.Options{
		LibcallPrefix: t.prefix,
		Variadic:      variadic,
	})
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	switch t.format {
	case FormatMachO:
		err = arm64macho.Write(&buf, o, arm64macho.Options{
			Platform: machocore.PlatformMacOS,
			MinOS:    dep,
		})
	case FormatELF:
		err = arm64elf.Write(&buf, o, arm64elf.Options{Comment: producer})
	default:
		err = fmt.Errorf("no %s writer for arm64", t.format)
	}
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func amd64Object(m *ir.Module, t Target, producer, dep string) ([]byte, error) {
	o, err := amd64lower.Lower(m, amd64lower.Options{
		LibcallPrefix: t.prefix,
	})
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	switch t.format {
	case FormatMachO:
		err = amd64macho.Write(&buf, o, amd64macho.Options{
			Platform: machocore.PlatformMacOS,
			MinOS:    dep,
		})
	case FormatELF:
		err = amd64elf.Write(&buf, o, amd64elf.Options{Comment: producer})
	case FormatPE:
		err = amd64pe.Write(&buf, o, amd64pe.Options{File: m.Name()})
	default:
		err = fmt.Errorf("no %s writer for amd64", t.format)
	}
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
