package objv

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vertex-language/elf"
	elflink "github.com/vertex-language/elf/link"
	"github.com/vertex-language/macho"
	macholink "github.com/vertex-language/macho/link"
	"github.com/vertex-language/pe"
	pelink "github.com/vertex-language/pe/link"

	"github.com/vertex-language/objv/sysroot"
)

// linkParams is one link, below VIR.
//
// The linker is objv's own: there is no cc on the path, nothing to detect and
// no host check. The three formats are three vertex-language linkers, each of
// which takes bytes and returns bytes, which is why an object never has to
// reach the filesystem to be linked.
type linkParams struct {
	// Objects are the images to link, in the order the caller gave them.
	// Order is significant and is preserved exactly.
	Objects []Input

	Output       string
	Entry        string
	Static       bool
	Freestanding bool

	// LibDirs and Libs are -L and -l; FrameworkDirs and Frameworks are -F
	// and -framework. A name is resolved to a file here rather than by the
	// linker, because only the PE linker has a search path of its own.
	//
	// Resolved libraries are added after every object, which is where a
	// linker expects them: an archive contributes only what something
	// already in the link needs.
	LibDirs       []string
	Libs          []string
	FrameworkDirs []string
	Frameworks    []string

	// The resolved sysroot: the SDK a Mach-O link reads its stubs from, the
	// platform's own directories, and the deployment target the image
	// records. It is the same Result the compilation used, passed along
	// rather than resolved again — the headers and the stub libraries have
	// to come from one SDK.
	Sysroot sysroot.Result
}

// link produces an executable from objects.
func link(t Target, p linkParams) error {
	if len(p.Objects) == 0 {
		return fmt.Errorf("nothing to link")
	}
	if p.Output == "" {
		return fmt.Errorf("link needs an output path")
	}
	// A framework is Apple's, and a target that has none cannot be given
	// one. Checked here rather than in the Mach-O path because the other
	// two would otherwise ignore -framework silently — and link without the
	// symbols it would have supplied, or succeed because nothing referenced
	// them.
	if len(p.Frameworks) > 0 && t.format != FormatMachO {
		return fmt.Errorf("cannot use -framework %s: %s has no frameworks",
			p.Frameworks[0], t.Name())
	}

	switch t.format {
	case FormatMachO:
		return linkMachO(t, p)
	case FormatELF:
		return linkELF(t, p)
	case FormatPE:
		return linkPE(t, p)
	}
	return fmt.Errorf("unsupported format %v", t.format)
}

func linkMachO(t Target, p linkParams) error {
	// The subtype is named rather than left zero. Mach-O's backend registry
	// keys on both halves and matches the subtype exactly, and only arm64's
	// "any implementation" subtype happens to be 0.
	var cpu macho.CPU
	var sub macho.SubCPU
	switch t.arch {
	case ArchAMD64:
		cpu, sub = macho.CPU_TYPE_X86_64, macho.CPU_SUBTYPE_X86_64_ALL
	case ArchARM64:
		cpu, sub = macho.CPU_TYPE_ARM64, macho.CPU_SUBTYPE_ARM64_ALL
	default:
		return fmt.Errorf("macho: unsupported arch %v", t.arch)
	}

	target := macho.Target{
		CPU:      cpu,
		SubCPU:   sub,
		Platform: macho.PlatformMacOS,
		Endian:   macho.LittleEndian,
	}
	if d := p.Sysroot.Deployment; !d.IsZero() {
		if v, err := macho.ParseVersion(d.String()); err == nil {
			target.MinOS = v
		}
	}

	l, err := macholink.New(target)
	if err != nil {
		return linkErr(err)
	}
	if p.Entry != "" {
		l.SetEntry(t.prefix + p.Entry)
	}

	// The SDK is sysroot's answer, not a second one: the include list and
	// the libSystem stub have to come from the same SDK.
	if !p.Freestanding && p.Sysroot.SDK.Found() {
		sdk := p.Sysroot.SDK.Path
		l.SetSDK(sdk)
		if data, err := os.ReadFile(filepath.Join(sdk, "usr/lib/libSystem.tbd")); err == nil {
			l.AddStub("libSystem", data)
		}
	}

	libs, err := p.libraries(t)
	if err != nil {
		return err
	}
	fws, err := p.frameworks()
	if err != nil {
		return err
	}
	if err := addObjects(l.AddFile, p.Objects); err != nil {
		return err
	}
	// Frameworks before the -l list, so a framework's own re-exports are in
	// hand before the C runtime is added at the end. Foundation re-exports
	// nothing objc needs, but a framework that depends on another does.
	if err := addObjects(l.AddFile, fws); err != nil {
		return err
	}
	if err := addObjects(l.AddFile, libs); err != nil {
		return err
	}

	img, err := l.Link()
	if err != nil {
		return linkErr(err)
	}
	b, err := img.Bytes()
	if err != nil {
		return linkErr(err)
	}
	return os.WriteFile(p.Output, b, 0o755)
}

func linkELF(t Target, p linkParams) error {
	var arch elf.Arch
	switch t.arch {
	case ArchAMD64:
		arch = elf.ArchAMD64
	case ArchARM64:
		arch = elf.ArchARM64
	default:
		return fmt.Errorf("elf: unsupported arch %v", t.arch)
	}
	target := elf.Target{Arch: arch, Class: elf.ELFCLASS64, Endian: elf.EndianLittle}

	l := elflink.New(target)
	if p.Entry != "" {
		l.SetEntry(p.Entry)
	}
	// Static is the linker's own switch here, and is set before any input is
	// added because it decides what an input is allowed to be.
	l.Options().Static = p.Static

	libs, err := p.libraries(t)
	if err != nil {
		return err
	}
	if err := addObjects(l.AddFile, p.Objects); err != nil {
		return err
	}
	if err := addObjects(l.AddFile, libs); err != nil {
		return err
	}
	img, err := l.Link()
	if err != nil {
		return linkErr(err)
	}
	return os.WriteFile(p.Output, img.Bytes(), 0o755)
}

func linkPE(t Target, p linkParams) error {
	var m pe.Machine
	switch t.arch {
	case ArchAMD64:
		m = pe.MachineAMD64
	case ArchARM64:
		m = pe.MachineARM64
	default:
		return fmt.Errorf("pe: unsupported arch %v", t.arch)
	}
	target := pe.Target{Machine: m, SubArch: m.SubArch(), ABI: pe.ABIMSVC, OS: pe.OSWindows}

	l, err := pelink.New(target)
	if err != nil {
		return linkErr(err)
	}
	if p.Entry != "" {
		l.SetEntry(p.Entry)
	}
	// The search path is handed over as well as used here, because the PE
	// linker resolves names of its own: a /DEFAULTLIB inside a CRT object
	// names a library nobody wrote on the command line.
	l.SetLibPath(p.libraryDirs()...)

	libs, err := p.libraries(t)
	if err != nil {
		return err
	}
	if err := addObjects(l.AddObject, p.Objects); err != nil {
		return err
	}
	if err := addObjects(l.AddArchive, libs); err != nil {
		return err
	}
	img, err := l.Link()
	if err != nil {
		return linkErr(err)
	}
	b, err := img.Bytes()
	if err != nil {
		return linkErr(err)
	}
	return os.WriteFile(p.Output, b, 0o755)
}

// libraryDirs is where a -l name is looked for: the caller's first.
func (p linkParams) libraryDirs() []string {
	dirs := make([]string, 0, len(p.LibDirs)+len(p.Sysroot.LibraryDirs))
	dirs = append(dirs, p.LibDirs...)
	return append(dirs, p.Sysroot.LibraryDirs...)
}

// frameworkDirs is where a -framework name is looked for: the caller's -F
// directories first, then the SDK's. They are the same directories the
// *headers* came from, which is the point — a framework's stub and its
// headers are one thing.
func (p linkParams) frameworkDirs() []string {
	dirs := append([]string(nil), p.FrameworkDirs...)
	for _, e := range p.Sysroot.Frameworks {
		dirs = append(dirs, e.Name)
	}
	return dirs
}

// frameworks resolves every -framework name for one link, in order.
func (p linkParams) frameworks() ([]Input, error) {
	if len(p.Frameworks) == 0 {
		return nil, nil
	}
	dirs := p.frameworkDirs()
	out := make([]Input, 0, len(p.Frameworks))
	for _, name := range p.Frameworks {
		fw, err := findFramework(name, dirs)
		if err != nil {
			return nil, err
		}
		out = append(out, fw)
	}
	return out, nil
}

// libraries resolves every -l name for one link, in order: the ones the
// caller named, then the platform's own runtime.
//
// The runtime comes last and comes always, because a program that says
// nothing about libraries still has to reach main and a program that does say
// something still has to. --freestanding is how a caller says none, and a
// runtime the caller already named is not added twice.
//
// Last is also where a static link wants it: an archive satisfies the
// references to its left, so the runtime after the libraries that call into
// it resolves and the reverse does not.
func (p linkParams) libraries(t Target) ([]Input, error) {
	names := p.libraryNames()
	if len(names) == 0 {
		return nil, nil
	}
	dirs := p.libraryDirs()
	out := make([]Input, 0, len(names))
	for _, name := range names {
		lib, err := findLibrary(t, name, dirs, p.Static)
		if err != nil {
			return nil, err
		}
		out = append(out, lib)
	}
	return out, nil
}

func (p linkParams) libraryNames() []string {
	names := append([]string(nil), p.Libs...)
	named := make(map[string]bool, len(names))
	for _, n := range names {
		named[n] = true
	}
	for _, n := range p.Sysroot.Libraries {
		if !named[n] {
			names = append(names, n)
		}
	}
	return names
}

// linkErr prefixes a linker's error with "link:" unless it already says so.
func linkErr(err error) error {
	if strings.HasPrefix(err.Error(), "link:") {
		return err
	}
	return fmt.Errorf("link: %w", err)
}

// addObjects hands each input to a linker's add function, in order.
//
// An input carrying bytes never touches the filesystem: the linkers take
// (name, data), so an object this process just produced goes straight in. One
// that names a path is read here, at the last possible moment.
func addObjects(add func(string, []byte) error, objs []Input) error {
	for _, o := range objs {
		data, err := o.bytes()
		if err != nil {
			return err
		}
		if err := add(o.Name, data); err != nil {
			return err
		}
	}
	return nil
}
