package objv

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// BuildParams is one build: what to compile, what to link it with, and where
// to put the result.
type BuildParams struct {
	// Output is the path the executable is written to. Required.
	Output string

	// Inputs are source files and object files in link order.
	Inputs []Input

	// Libraries and LibraryDirs specify -l and -L options.
	// Frameworks and FrameworkDirs specify -framework and -F options (Darwin).
	Libraries     []string
	LibraryDirs   []string
	Frameworks    []string
	FrameworkDirs []string

	// Entry overrides the entry symbol. "" is the platform's default.
	Entry string

	// Static links archives only: no dynamic loader, no shared objects.
	Static bool
}

// Build compiles all source inputs and links the resulting executable.
func (c *Compiler) Build(p BuildParams) error {
	if p.Output == "" {
		return errors.New("build needs an output path")
	}
	t, err := c.target()
	if err != nil {
		return err
	}
	if err := t.Supports(); err != nil {
		return err
	}

	objs, diags, err := c.compileAll(p.Inputs)
	if err != nil {
		return err
	}
	if HasErrors(diags) {
		return &DiagnosticError{Diagnostics: diags}
	}

	_, r, err := c.config()
	if err != nil {
		return err
	}
	return link(t, linkParams{
		Objects:       objs,
		Output:        p.Output,
		Entry:         p.Entry,
		Static:        p.Static,
		Freestanding:  c.Freestanding,
		LibDirs:       p.LibraryDirs,
		Libs:          p.Libraries,
		FrameworkDirs: append(append([]string(nil), c.FrameworkDirs...), p.FrameworkDirs...),
		Frameworks:    p.Frameworks,
		Sysroot:       r,
	})
}

// Compile compiles every source input to an object file beside it, and
// returns the paths. Non-source inputs are returned unchanged, so the result
// is a link list.
//
// It is `objv build -c`: the half of Build that stops before the linker.
func (c *Compiler) Compile(inputs []Input, dir string) ([]string, []Diagnostic, error) {
	objs, diags, err := c.compileAll(inputs)
	if err != nil {
		return nil, diags, err
	}
	if HasErrors(diags) {
		return nil, diags, &DiagnosticError{Diagnostics: diags}
	}
	out := make([]string, 0, len(objs))
	for _, o := range objs {
		if o.Data == nil {
			out = append(out, o.Name)
			continue
		}
		path := filepath.Join(dir, filepath.Base(o.Name))
		if err := os.WriteFile(path, o.Data, 0o644); err != nil {
			return nil, diags, err
		}
		out = append(out, path)
	}
	return out, diags, nil
}

// compileAll turns a mixed input list into a link list.
//
// Compilation continues past a unit with errors, so that a build of four
// files reports all four files' mistakes rather than the first one's. The
// caller checks HasErrors before linking.
func (c *Compiler) compileAll(inputs []Input) ([]Input, []Diagnostic, error) {
	if len(inputs) == 0 {
		return nil, nil, errors.New("no inputs")
	}
	var diags []Diagnostic
	out := make([]Input, 0, len(inputs))
	for _, in := range inputs {
		if !in.isSource() {
			out = append(out, in)
			continue
		}
		obj, ds, err := c.Object(in)
		diags = append(diags, ds...)
		if err != nil {
			return nil, diags, fmt.Errorf("%s: %w", in.name(), err)
		}
		if obj == nil {
			continue // reported; keep going so the next file is read too
		}
		out = append(out, ObjectBytes(objectName(in), obj))
	}
	return out, diags, nil
}

// objectName is what an object produced in memory is called in the link and
// in a linker's diagnostics.
func objectName(in Input) string {
	base := filepath.Base(in.name())
	return base[:len(base)-len(filepath.Ext(base))] + ".o"
}
