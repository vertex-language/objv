// Package objv is the Vertex Objective-C compiler.
//
// It coordinates target configuration, preprocessing, parsing, semantic analysis,
// lowering to VIR, and linking. Individual pipeline phases are also accessible directly
// (Env, Preprocess, Parse, Check, Module, Object, Build).
// Diagnostic positions across original headers and preprocessed output are mapped back
// to their original source sites.
package objv

// Version is the compiler's own version, stamped into objects and printed by
// `objv version`.
const Version = "0.1.0"
