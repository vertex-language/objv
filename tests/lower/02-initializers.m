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

// A struct with no tag still needs a name in VIR, where every struct type
// has one. It is numbered, and the number is not written with a '.': a type
// name is written bare in the IR text and has to be an identifier there,
// where a symbol goes through a mapping that accepts one.
struct { int x, y; } point = { 3, 4 };
// vir: internal type @anon_
// vir: export global rw @_point @anon_

// Nested braces. §6.7.9p17: the object is descended into, brace by brace,
// and the items inside an object's own braces belong to its *subobjects* —
// not to the object again. Reading the first item's braces as another
// initializer for the whole array leaves everything after the first scalar
// at zero, which compiles and runs and is wrong.
int grid[2][2] = { { 1, 2 }, { 3, 4 } };
// vir: export global rw @_grid [2][2]i32
// vir: = { { 1, 2 }, { 3, 4 } }

struct Inner pairs[2] = { { 1, 2 }, { 3, 4 } };
// vir: export global rw @_pairs [2]@struct_Inner

int nested(void) {
    struct Inner local[2] = { { 5, 6 }, { 7, 8 } };
    return local[1].c;
}

// A designator names where to write, and it may name a member the positional
// walk has already passed — so the bound is checked after the designator and
// not before it.
struct Back { int x, y; };
struct Back back = { .y = 7, .x = 3 };
// vir: export global rw @_back @struct_Back
// vir: = { 3, 7 }

int backLocal(void) {
    struct Back b = { .y = 7, .x = 3 };
    return b.x;
}
