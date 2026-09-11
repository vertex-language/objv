// Structs and unions across a call boundary.
//
// No calling convention passes one in a register, and what each does instead
// is a classification — AAPCS64 asks whether the aggregate is homogeneous,
// then whether it is sixteen bytes or less; SysV sorts each eightbyte into
// INTEGER, SSE or MEMORY. None of that is in lower. VIR states the question
// in the signature, with `byval` on a pointer parameter and `sret` on the
// first, and the backend answers it per target.

struct Pair { long a, b; };
struct Big { long v[8]; };

// A result is storage the caller allocated, and the function writes through
// the pointer rather than returning a value: its result list is empty.
struct Pair mk(long a, long b) { struct Pair p; p.a = a; p.b = b; return p; }
// vir: export func @_mk(%__ret ptr sret @struct_Pair, %a i64, %b i64) {
// vir-not: @_mk(%__ret ptr sret @struct_Pair, %a i64, %b i64) @struct_Pair

// An argument is a pointer to a copy. The copy is not optional: a by-value
// parameter is the callee's own object and it may assign to it, which is
// what this one does.
long take(struct Pair p, long t) { p.a = 99; return p.a * 100 + p.b * 10 + t; }
// vir: export func @_take(%p ptr byval @struct_Pair, %t i64) i64

long use(void) {
    struct Pair p = mk(3, 4);
    return take(p, 5);
}
// The caller allocates both the result's storage and the argument's copy.
// vir: memcpy

// Sixty-four bytes, which every convention returns through memory — and the
// same sret says so, because deciding that is the backend's job and not a
// second opinion arrived at here.
struct Big fill(long a) { struct Big r; for (int i = 0; i < 8; i++) r.v[i] = a + i; return r; }
// vir: export func @_fill(%__ret ptr sret @struct_Big, %a i64) {

// A method is a function of self and _cmd, so a method returning a struct is
// the same shape with the hidden pointer in front of both.
@interface Geo : NSObject
- (struct Pair)pair;
- (long)sum:(struct Pair)p;
@end

@implementation Geo
- (struct Pair)pair { return mk(1, 2); }
- (long)sum:(struct Pair)p { return p.a + p.b; }
@end
// vir: internal func @__i_Geo__pair(%__ret ptr sret @struct_Pair, %self ptr, %_cmd ptr) {
// vir: internal func @__i_Geo__sum_(%self ptr, %_cmd ptr, %p ptr byval @struct_Pair) i64

long sends(Geo *g) { return [g sum:[g pair]]; }
// A send carries the same attributes, on the trampoline's type: the hidden
// pointer goes in front of the receiver, which is where objc_msgSend_stret
// wants it on x86-64 and where X8 puts it on AArch64.
// vir: @msgsig_ptr_sret_struct_Pair_ptr_ptr
// vir: @msgsig_ptr_ptr_ptr_byval_struct_Pair

// A member of a call's result reads from the storage the call wrote into.
// §6.5.2.3 does not make it an lvalue — `pair().a = 1` is still refused —
// but it has to read from somewhere.
long first(Geo *g) { return [g pair].a; }
