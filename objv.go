// Package objv is the Vertex Objective-C compiler.
//
// It is the coordinator: the packages beneath it each do one job, and this
// one decides what a target means, composes phase 4's configuration from
// three sources that each know part of it, runs the front end, and hands what
// comes out to a backend.
//
//	var c objv.Compiler
//	err := c.Build(objv.BuildParams{
//		Output: "app",
//		Inputs: []objv.Input{objv.File("main.m")},
//		Frameworks: []string{"Foundation"},
//	})
//
// Every rung of the pipeline is also a method, because a tool usually wants
// one of them rather than the whole thing: Env resolves the configuration and
// stops, Preprocess is `--emit mi`, Parse hands back a tree, Check reports
// diagnostics, Module is `--emit vir`, Object is `--emit obj`, and Build
// links.
//
// # Where a diagnostic comes from
//
// Three position spaces meet here, which is why Diagnostic carries a Site
// rather than a line number. Phase 4 reports across every file the include
// graph reached — hundreds of them behind one #import. Phases 5 through 7
// report in the *preprocessed* text, which is a different file from the one
// the user wrote. Both are mapped back before they leave this package.
package objv

// Version is the compiler's own version, stamped into objects and printed by
// `objv version`.
const Version = "0.1.0"
