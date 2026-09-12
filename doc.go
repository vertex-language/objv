package objv

// The pipeline, and where each decision is made.
//
//	                        objv (this package)
//	                   target table · predefines · feature answers
//	                                  │
//	  Input ──▶ preprocessor ──▶ print ──▶ parser ──▶ analyzer ──▶ lower ──▶ ir/lower ──▶ link
//	             phase 4       reparse     ast        types        VIR       isel        image
//	                │            │
//	           sysroot          srcMap ──────── every diagnostic maps back through this
//	        SDK · frameworks
//	        deployment target
//
// Three responsibilities are composed here:
//
// 1. Target configuration: Maps target triples to a type model, runtime ABI,
// and backend architecture/container in a unified table (target.go).
//
// 2. Predefined macros: Composes language predefines (preprocessor), platform
// macros (sysroot), and target-dependent limits/sizes (types.Model).
//
// 3. Reparse bridge: Translates preprocessor token streams across include graphs
// into parsed ASTs, tracking source maps so diagnostics map back to original sites.
