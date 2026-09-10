// #pragma pack, which is a phase-7 pragma and not a phase-4 one.
//
// Phase 4 forwards the pragmas it does not act on, deliberately, because
// some of them mean something further down. This one caps every member's
// alignment in the structures declared after it, and Apple's
// <mach/message.h> wraps three hundred lines in it — every message trailer
// there has a size that is wrong without it.

typedef unsigned int u32;
typedef unsigned long long u64;

#pragma pack(push, 4)
struct trailer { u32 a; u32 b; u32 c; u64 ctx; };
_Static_assert(sizeof(struct trailer) == 20, "capped to 4");
#pragma pack(pop)

struct plain { u32 a; u32 b; u32 c; u64 ctx; };
_Static_assert(sizeof(struct plain) == 24, "natural alignment");

// The ceiling is a property of the point the struct was written at, so a
// nested push and pop restore what was in force.
#pragma pack(push, 2)
struct two { u32 a; u64 b; };
_Static_assert(sizeof(struct two) == 12, "capped to 2");
#pragma pack(push, 8)
struct eight { u32 a; u64 b; };
_Static_assert(sizeof(struct eight) == 16, "8 is not a cap here");
#pragma pack(pop)
struct two_again { u32 a; u64 b; };
_Static_assert(sizeof(struct two_again) == 12, "back to 2");
#pragma pack(pop)

struct plain_again { u32 a; u64 b; };
_Static_assert(sizeof(struct plain_again) == 16, "no ceiling");

// The globals prove the layout reached lowering: a global's ftype names a
// struct type whose every field carries the offset the analyzer measured,
// and the two structs differ only in where ctx lands.
struct trailer packed_one;
// vir: export global rw @_packed_one @struct_trailer
// vir: ctx i64 at 12

struct plain plain_one;
// vir: export global rw @_plain_one @struct_plain
// vir: ctx i64 at 16
