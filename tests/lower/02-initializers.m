// Initializers, at both scopes.
//
// A file-scope initializer is a value the module carries; an automatic one
// is code. The two walks are separate and the corpus checks both, because
// the same brace elision and the same designators have to work in each.

struct Inner { int b, c; };
struct Outer { int a; struct Inner d; char name[4]; };

int table[4] = { 1, 2, 3, 4 };
// vir: export global rw @_table [4]i32
// vir: = { 1, 2, 3, 4 }

struct Outer origin = { .a = 1, .d.c = 7, .name = "ok" };
// vir: internal type @struct_Outer struct
// vir: export global rw @_origin @struct_Outer

const char *greeting = "hello";
// vir: export global rw @_greeting ptr
// vir: [6]i8

double ratios[2] = { 0.5, 1.5 };
// vir: [2]f64

int locals(void) {
    // A braced initializer zeroes what it does not mention (§6.7.9p21),
    // which is one memset rather than a store per hole.
    int v[4] = { 1, 2, [3] = 9 };
    struct Outer s = { 1, 2, 3, "hi" };
    char msg[8] = "hey";
    static int once = 5;
    return v[3] + s.d.c + msg[0] + once;
}
// vir: memset
// vir: memcpy
// vir: internal global rw @_static$once
