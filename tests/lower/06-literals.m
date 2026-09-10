// §6.8's literals: syntax that means an object nobody wrote a send for.
//
// All but one of them are sends. @"…" is the exception — it has to work
// before any class is realized, so it is data in the image and not a call.

id text(void) { return @"hi"; }
// The isa is CoreFoundation's, which is what lets a constant string exist
// in a process that never loaded Foundation.
// vir: import global @___CFConstantStringClassReference
// vir: section "__DATA,__cfstring"
// vir: 1992
// vir: __TEXT,__cstring,cstring_literals

id wide(void) { return @"héllo"; }
// A literal with one non-ASCII character changes encoding, section and
// count all at once.
// vir: 2000
// vir: __TEXT,__ustring

id numbers(void) { return @42; }
// vir: "numberWithInt:"

id flag(void) { return @__objc_yes; }
// vir: "numberWithBool:"

id real(void) { return @(1.5); }
// vir: "numberWithDouble:"

id fromC(const char *s) { return @(s); }
// vir: "stringWithUTF8String:"

id list(id a, id b) { return @[a, b]; }
// The elements go into a frame array: arrayWithObjects:count: has no
// varargs form for the compiler to use.
// vir: "arrayWithObjects:count:"

id map(id k, id v) { return @{ k : v }; }
// vir: "dictionaryWithObjects:forKeys:count:"
