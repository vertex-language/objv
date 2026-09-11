package objv

import (
	"github.com/vertex-language/ir"
	"github.com/vertex-language/ir/text"

	"github.com/vertex-language/objv/lower"
	"github.com/vertex-language/objv/runtime"
)

// Module lowers one input to VIR — the artifact `--emit vir` writes.
//
// The module is never nil, even when diagnostics come back: a partial module
// is what a caller inspecting broken input should get. Whether to use it is
// the caller's call, and HasErrors is how that call is made.
func (c *Compiler) Module(in Input) (*ir.Module, []Diagnostic, error) {
	u, err := c.frontend(in)
	if err != nil {
		return nil, nil, err
	}
	defer u.release()

	if u.failed() {
		// Lowering a unit the analyzer rejected produces internal errors
		// about a tree that was already reported as wrong, which buries the
		// diagnostic that matters under diagnostics that do not.
		return nil, c.report(u.diagnostics()), nil
	}
	m := c.lowerUnit(u)
	return m, c.report(u.diagnostics()), nil
}

// VIR renders one input as VIR text.
func (c *Compiler) VIR(in Input) ([]byte, []Diagnostic, error) {
	m, diags, err := c.Module(in)
	if err != nil {
		return nil, nil, err
	}
	if m == nil {
		return nil, diags, nil
	}
	out, err := text.Format(m)
	if err != nil {
		return nil, diags, err
	}
	return out, diags, nil
}

// Object compiles one input to an object file's bytes — `--emit obj`.
func (c *Compiler) Object(in Input) ([]byte, []Diagnostic, error) {
	m, diags, err := c.Module(in)
	if err != nil {
		return nil, nil, err
	}
	if m == nil || HasErrors(diags) {
		return nil, diags, nil
	}
	t, _ := c.target()
	_, r, _ := c.config()
	dep := ""
	if !r.Deployment.IsZero() {
		dep = r.Deployment.String()
	}
	obj, err := emitObject(m, t, c.producer(), dep)
	if err != nil {
		return nil, diags, err
	}
	return obj, diags, nil
}

// deployment is the oldest OS this build runs on, which is what §6.10's
// @available compares against. The sysroot settled it; this only reads it
// back in the shape lower wants.
func (c *Compiler) deployment() runtime.OSVersion {
	_, r, _ := c.config()
	return runtime.OSVersion{
		Major: r.Deployment.Major,
		Minor: r.Deployment.Minor,
		Patch: r.Deployment.Patch,
	}
}

// lowerUnit runs the last phase, appending its diagnostics to the unit's.
//
// It is separate from Module so that Build can lower without rendering, and
// so that the one place lower.Options is filled in is the one place a target
// becomes lowering configuration.
func (c *Compiler) lowerUnit(u *unit) *ir.Module {
	m, diags := lower.Lower(u.file, u.tree, u.info, lower.Options{
		Name:         u.in.moduleName(),
		Target:       u.target.IR(),
		Model:        u.target.Model(),
		ABI:          u.target.ABI(),
		Arch:         u.target.RuntimeArch(),
		Platform:     u.target.Platform(),
		Deployment:   c.deployment(),
		ARC:          c.ARC,
		SymbolPrefix: u.target.SymbolPrefix(),
	})
	u.diags = append(u.diags, diags...)
	return m
}
