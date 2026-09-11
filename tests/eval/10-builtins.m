// The compiler's own functions.
//
// A builtin names an instruction, not a symbol: nothing here calls anything,
// and the point of each marker below is that the verb appears with no `call`
// beside it. The three widths share one text because the dispatch is on the
// lowered value's type and not on the name's suffix.

double fabs_d(double x) { return __builtin_fabs(x); }
// vir: f64.abs
float fabs_f(float x) { return __builtin_fabsf(x); }
// vir: f32.abs

double roots(double x, float y) { return __builtin_sqrt(x) + __builtin_sqrtf(y); }
// vir: f64.sqrt
// vir: f32.sqrt

double edges(double x) {
    return __builtin_ceil(x) + __builtin_floor(x) + __builtin_trunc(x);
}
// vir: f64.ceil
// vir: f64.floor
// vir: f64.trunc

double pick(double a, double b) {
    // fmin and fmax return the other operand when one is a NaN, which VIR
    // spells minnum and maxnum; minimum and maximum propagate it and are a
    // different function.
    return __builtin_fmin(a, b) + __builtin_fmax(a, b) + __builtin_copysign(a, b);
}
// vir: f64.minnum
// vir: f64.maxnum
// vir: f64.copysign
// vir-not: f64.minimum

double scaled(double a, double b, double c) { return __builtin_fma(a, b, c); }
// vir: f64.fma

double infinite(void) { return __builtin_inf() + __builtin_huge_val(); }
// vir: f64.const inf

int bits(unsigned a, unsigned long long b) {
    return __builtin_clz(a) + __builtin_ctz(a) + __builtin_popcount(a) +
           __builtin_parityll(b) + __builtin_popcountll(b);
}
// vir: i32.clz
// vir: i32.ctz
// vir: i32.popcnt
// vir: i64.popcnt
// A 64-bit count still fits in an int, so the result is narrowed.
// vir: i32.wrap_i64

unsigned reversed(unsigned x) { return __builtin_bswap32(x); }
// vir: i32.bswap

unsigned short reversed16(unsigned short x) { return __builtin_bswap16(x); }
// VIR has no i16: a 16-bit value is held zero-extended in an i32, so
// reversing all four bytes leaves the two that matter in the top half.
// vir: i32.ushr

long likely(long c) { return __builtin_expect(c != 0, 1); }
// The hint has nowhere to go — VIR carries no branch weights — and the value
// of the builtin is its first argument in any case.
// vir-not: expect

int known(int x) { return __builtin_constant_p(x); }
// The answer is no, and the argument is not evaluated.
// vir: i32.const 0

int known_const(void) { return __builtin_constant_p(2 + 3); }
// And yes when the operand folds: the analyzer answers it in the one
// constant evaluator this compiler has, so the two phases cannot disagree.
// vir: i32.const 1

int stops(int x) {
    if (x > 0) return x;
    __builtin_unreachable();
}
// C says reaching an unreachable is undefined and VIR has no verb for
// undefined behaviour under a friendlier name, so the path ends in a trap.
// vir: trap

// §7.17's read-modify-write operations, which objv's own <stdatomic.h> is
// written in terms of. They have no signature — the type is the pointee of
// the first argument, and one name serves every width — so the analyzer
// types them from the operand and lowering picks the verb the same way.
long bumpLong(long *p, long by);
long bumpLong(long *p, long by) { return __builtin_atomic_fetch_add(p, by); }
int swapInt(int *p, int v);
int swapInt(int *p, int v) { return __builtin_atomic_exchange(p, v); }
_Bool claim(long *p, long *want, long desired);
_Bool claim(long *p, long *want, long desired) {
    return __builtin_atomic_compare_exchange(p, want, desired);
}
// vir: i64.atomic_rmwadd
// vir: i32.atomic_rmwxchg
// vir: i64.atomic_cas
