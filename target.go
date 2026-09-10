package objv

import (
	"errors"
	"runtime"
	"sort"
	"strings"

	"github.com/vertex-language/ir"

	"github.com/vertex-language/objv/preprocessor"
	objcrt "github.com/vertex-language/objv/runtime"
	"github.com/vertex-language/objv/types"
)

// An Arch is a backend in ir/lower.
type Arch uint8

const (
	ArchAMD64 Arch = iota
	ArchARM64
)

func (a Arch) String() string {
	switch a {
	case ArchAMD64:
		return "amd64"
	case ArchARM64:
		return "arm64"
	}
	return "invalid"
}

// A Format is an object file container.
type Format uint8

const (
	FormatMachO Format = iota
	FormatELF
	FormatPE
)

func (f Format) String() string {
	switch f {
	case FormatMachO:
		return "macho"
	case FormatELF:
		return "elf"
	case FormatPE:
		return "pe"
	}
	return "invalid"
}

// ldblKind names a long double format. types.Model carries sizes, and size
// alone cannot tell x87 extended from IEEE quad — both occupy sixteen bytes —
// so the format is a fact of the target table, beside the Model rather than
// inside it: layout never needs it and float.h's predefines do.
type ldblKind uint8

const (
	ldblDouble ldblKind = iota // long double == double (arm64 macOS)
	ldblX87                    // 80-bit extended in 16 bytes (x86-64)
	ldblQuad                   // IEEE binary128 (aarch64 Linux)
)

// A Target is everything a target name decides, in one object.
//
// "aarch64-macos" says three things to three parts of the compiler. To the
// front end it is a type model: how wide a long is, what float.h says about
// long double. To the Objective-C half it is a runtime ABI: which metadata
// layout, which objc_msgSend variant, which sections. Below VIR it is an
// architecture, a container format, and the prefix a C identifier wears in
// the symbol table.
//
// All three are here, because the caller who needs one usually needs the
// others, and because holding them in three places is what forces a name to
// be looked up three times and answered differently twice.
type Target struct {
	name string

	// The front end's half.
	model types.Model
	ldbl  ldblKind
	wint  string // C type of wint_t: glibc says unsigned int, Darwin says int

	// The Objective-C half.
	abi    objcrt.ABI
	rtArch objcrt.Arch

	// The machine's half.
	arch   Arch
	format Format
	irt    ir.Target
	prefix string // "_" on Mach-O, empty on ELF; lower.Options applies it

	// unsupported, when non-empty, is why this target names a machine no hop
	// below VIR is written for. It is data rather than a discovery, so a
	// caller can ask before compiling instead of learning four phases in.
	unsupported string
}

func (t Target) Name() string             { return t.name }
func (t Target) Model() types.Model       { return t.model }
func (t Target) Arch() Arch               { return t.arch }
func (t Target) Format() Format           { return t.format }
func (t Target) IR() ir.Target            { return t.irt }
func (t Target) SymbolPrefix() string     { return t.prefix }
func (t Target) ABI() objcrt.ABI          { return t.abi }
func (t Target) RuntimeArch() objcrt.Arch { return t.rtArch }

// Supports reports whether objv can build for this target, and says why not
// when it cannot. A Target names a machine; it does not promise every hop
// below VIR is written for it.
func (t Target) Supports() error {
	if t.unsupported != "" {
		return errors.New(t.unsupported)
	}
	return nil
}

// darwinARM64 is LP64 with the one Darwin difference: long double is double
// on Apple Silicon, where every other 64-bit target gives it more.
func darwinARM64() types.Model {
	m := types.LP64()
	m.SizeLongDouble, m.AlignLongDouble = 8, 8
	return m
}

// targets is every target objv models, and is the authority on the name.
//
// An entry exists when objv can state all three halves. The table does not
// grow on request; it grows with a Model, a runtime ABI and a backend to
// back it.
var targets = map[string]Target{
	"aarch64-macos": {
		model: darwinARM64(), ldbl: ldblDouble, wint: "int",
		abi: objcrt.Darwin64(), rtArch: objcrt.ARM64,
		arch: ArchARM64, format: FormatMachO, irt: ir.AArch64MacOS,
		prefix: "_",
	},
	"x86_64-macos": {
		model: types.LP64(), ldbl: ldblX87, wint: "int",
		abi: objcrt.Darwin64(), rtArch: objcrt.AMD64,
		arch: ArchAMD64, format: FormatMachO, irt: ir.X86_64MacOS,
		prefix: "_",
	},
	"aarch64-linux": {
		model: types.LP64(), ldbl: ldblQuad, wint: "unsigned int",
		abi:    objcrt.ABI{Kind: objcrt.GNUstep, Container: objcrt.ELF, PtrBytes: 8},
		rtArch: objcrt.ARM64,
		arch:   ArchARM64, format: FormatELF, irt: ir.AArch64Linux,
	},
	"x86_64-linux": {
		model: types.LP64(), ldbl: ldblX87, wint: "unsigned int",
		abi:    objcrt.ABI{Kind: objcrt.GNUstep, Container: objcrt.ELF, PtrBytes: 8},
		rtArch: objcrt.AMD64,
		arch:   ArchAMD64, format: FormatELF, irt: ir.X86_64Linux,
	},
	"x86_64-windows": {
		model: llp64(), ldbl: ldblDouble, wint: "unsigned short",
		abi:    objcrt.ABI{Kind: objcrt.GNUstep, Container: objcrt.COFF, PtrBytes: 8},
		rtArch: objcrt.AMD64,
		arch:   ArchAMD64, format: FormatPE, irt: ir.X86_64Windows,
		unsupported: "objv has no PE object writer yet; --emit vir works, --emit obj does not",
	},
}

// llp64 is Windows: a 32-bit long under 64-bit pointers, and MSVC's
// bit-field rule, which is an ABI rather than a preference.
func llp64() types.Model {
	m := types.LP64()
	m.SizeLong = 4
	m.WCharKind = types.UShort
	m.SizeLongDouble, m.AlignLongDouble = 8, 8
	m.MSBitfields = true
	return m
}

// LookupTarget is how a target name becomes a Target.
func LookupTarget(name string) (Target, bool) {
	t, ok := targets[name]
	if !ok {
		return Target{}, false
	}
	t.name = name
	return t, true
}

// Targets is every target objv models, sorted.
func Targets() []string {
	out := make([]string, 0, len(targets))
	for n := range targets {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// HostTarget is the machine objv is running on. It is not a Target on a
// machine objv does not model, which is where every verb that needs one says
// so.
func HostTarget() (Target, bool) { return LookupTarget(HostName()) }

// HostName is the target name for the machine objv is running on, or "" where
// that machine is not one objv models. The Go spellings are not C's, which is
// what the two tables here translate.
func HostName() string {
	arch := map[string]string{"amd64": "x86_64", "arm64": "aarch64"}[runtime.GOARCH]
	osname := map[string]string{"darwin": "macos", "linux": "linux", "windows": "windows"}[runtime.GOOS]
	if arch == "" || osname == "" {
		return ""
	}
	return arch + "-" + osname
}

// SplitTarget splits a target name into its architecture and OS halves.
func SplitTarget(name string) (arch, osname string) {
	if i := strings.IndexByte(name, '-'); i >= 0 {
		return name[:i], name[i+1:]
	}
	return name, ""
}

func targetList() string { return strings.Join(Targets(), ", ") }

// Triple is the target named the way a target triple names one, which is what
// TargetConditionals.h's __is_target_* operators compare against.
//
// objv's own target names are two words — aarch64-macos — because two is what
// selecting a backend needs. A triple has four, and the other two are facts
// this table knows rather than guesses: Apple is the vendor of every Darwin
// target, and no target objv models is in a Catalyst or simulator
// environment, so the environment is empty and every question about one is
// answered no.
//
// The architecture is spelled Apple's way on Darwin. TargetConditionals.h
// asks `__is_target_arch(arm64)`, never aarch64, and a triple that answered
// the architecture's own name would decide TARGET_OS_OSX is 0 — silently,
// because the header falls through to a default rather than complaining.
func (t Target) Triple() preprocessor.Triple {
	arch, osname := SplitTarget(t.name)
	tr := preprocessor.Triple{Arch: arch, OS: osname}
	switch osname {
	case "macos":
		tr.Vendor = "apple"
		if arch == "aarch64" {
			tr.Arch = "arm64"
		}
	case "linux":
		tr.Vendor = "unknown"
		tr.Environment = "gnu"
	case "windows":
		tr.Vendor = "pc"
		tr.Environment = "msvc"
	}
	return tr
}
