// Bit-fields.
//
// §6.7.2.1p13 gives a bit-field no address, which is the whole of why it is
// different: every other member is read by computing an address and loading
// through it, and a bit-field by loading the allocation unit it shares with
// its neighbours and shifting the bits out. Writing one is that backwards —
// a read, a replacement, and a store, for what the source wrote as an
// assignment.

struct F {
    unsigned a : 3;
    unsigned b : 5;
    int      c : 6;
    unsigned d;
};

unsigned reads(struct F *f) { return f->a; }
// The unit is loaded whole and the bits shifted out of it. An unsigned field
// is masked by shifting back down without sign extension.
// vir: i32.shl
// vir: i32.ushr
// vir-not: ptr.add %f_addr

int signedRead(struct F *f) { return f->c; }
// A signed field's top bit is its sign, so it goes to bit 31 and comes back
// arithmetically.
// vir: i32.sshr

void writes(struct F *f, unsigned v) { f->a = v; }
// A read, the bits replaced, and a store: the neighbours in the unit have to
// come back unchanged.
// vir: i32.and
// vir: i32.or

int bump(struct F *f) { return ++f->c; }
// vir: i32.add

// A file-scope initializer is a value, so there is no read-modify-write to
// emit: the bits are packed into the bytes here.
struct F g = { 1, 2, -3, 4 };
// The three fields share one unit, and the byte it lands in holds
// 1 | (2 << 3) = 17.
// vir: export global rw @_g @struct_F
// vir: 17

// The VIR type describes the bytes rather than the fields, because there is
// nothing to name: a bit-field has no address for a field to be the address
// of.
// The range is the bytes the *bits* occupy and not the allocation unit's:
// a unit may overlap an ordinary member, and two fields of a struct type may
// not overlap. Three fields of 3, 5 and 6 bits are fourteen bits, so two
// bytes.
// vir: bits_0 [2]i8

// A field wider than an int keeps its declared type rather than promoting,
// and one past bit 32 of its unit is reached only by loading the whole unit
// — so both are shifted at sixty-four bits.
struct W { unsigned long long q : 40; unsigned long long r : 20; };
unsigned long long high(struct W *w) { return w->r; }
// vir: i64.shl
// vir: i64.ushr
