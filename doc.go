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
// Three things are composed here and nowhere else.
//
// **A target is three answers, not one.** "aarch64-macos" is a type model to
// the front end, a runtime ABI to the Objective-C half, and an architecture
// plus a container to the backend. Holding them in three packages is what
// forces a name to be looked up three times and answered differently twice,
// so Target holds all three and target.go is the only table.
//
// **The predefined macros come from three places**, because three different
// things know them: the language's are the preprocessor's (__OBJC__,
// __STDC__), the platform's are sysroot's (__APPLE__, __LITTLE_ENDIAN__, the
// deployment target), and the type model's are here. None of the three could
// compute another's without importing something it must not.
//
// **The reparse bridge** is the one piece of plumbing with no other reason to
// exist. Phase 4's output spans every file the include graph reached and
// parser.ParseFile takes one *token.File, so the stream is printed and
// re-scanned — the round trip --emit mi already promises, used as machinery.
// The cost is that every position phases 5 through 7 report is in the printed
// text, which is why a srcMap is recorded as it is printed and why Diagnostic
// carries a Site rather than a line number.
