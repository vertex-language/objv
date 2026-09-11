// C functions: what a translation unit with no Objective-C in it becomes.
//
// The whole substrate is here — arithmetic across widths, the control flow
// that becomes blocks, and the two shapes a call can take — because these
// are what every method body is made of.

int twice(int x) { return x * 2; }
// vir: export func @_twice(%x i32) i32
// vir: i32.mul

// A static function is emitted when this unit calls it, and not otherwise.
// The symbol is internal, so nothing outside the unit can reach it, and a
// definition nobody here names is text in the object with no way in. That
// rule is not an optimisation: it is what makes a header full of `static
// inline` definitions -- Apple's <math.h> is dozens of them -- cost only
// what the program actually uses.
static int hidden(int x) { return x + 1; }
int reaches_hidden(int x) { return hidden(x); }
// vir: internal func @_hidden

static int never_called(int x) { return x - 1; }
// vir-not: @_never_called

long widen(int a, unsigned b, long c) {
    // §6.3.1.8: both operands reach their common type before the operator.
    return a + c + (long)(b * 2u);
}
// vir: i64.sext_i32
// vir: i64.zext_i32

int loops(int n) {
    int total = 0;
    for (int i = 0; i < n; i++) {
        if (i == 3) continue;
        if (i == 7) break;
        total += i;
    }
    while (total > 100) total -= 10;
    do { total++; } while (total < 0);
    switch (total) {
    case 1: return 1;
    case 2: return 2;
    default: break;
    }
    return total;
}
// vir: @for_cond
// vir: @for_post
// vir: @while_cond
// vir: @do_body
// VIR's br_table selects by index and a C switch is sparse, so the values
// are compared one at a time.
// vir: @switch_case
// vir: @switch_done

int shifts(long v, int n) { return (int)((v << n) >> 3); }
// vir: i64.shl
// vir: i64.sshr

typedef int (*binop)(int);

int indirect(binop f) { return f(3) + (*f)(4); }
// vir: callind
// vir: type @fnsig_i32_ri32 func (i32) i32

int nulls(const char *p) { return p == 0 || p == (void *)0; }
// vir: ptr.const 0
// vir: ptr.eq
// A null pointer constant is the null pointer, not an address computed by
// adding zero to one.
// vir-not: ptr.add

// A function defined further down the file, called from one lowered above
// it: every definition's signature is built before any body, because a call
// has to see the arity it will have and parameters may only be added to a
// function before its entry block exists.
static void later(int a, int b);
int earlier(int n);
int earlier(int n) { later(n, n + 1); return n; }
static void later(int a, int b) { (void)a; (void)b; }
// vir: call @_later(
