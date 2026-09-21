// setjmp's relatives -- sigsetjmp/siglongjmp and _setjmp/_longjmp -- in a
// helper function rather than main, jumped to from deep recursion. Only
// what C defines is read back: volatile locals changed after the setjmp,
// and locals not changed since it.
#include <stdio.h>
#include <setjmp.h>

static sigjmp_buf sig_env;
static jmp_buf raw_env;

static void dive(int n, int code) {
    if (n == 0) siglongjmp(sig_env, code);
    dive(n - 1, code);
}

static int attempts(int limit) {
    int fixed = limit * 10;
    volatile int count = 0;
    volatile double total = 0.5;
    int code = sigsetjmp(sig_env, 1);
    count++;
    total += code;
    if (code < limit) dive(20, code + 1);
    return (int)(count * 100 + total * 10) + fixed;
}

static int raw(void) {
    volatile int hops = 0;
    if (_setjmp(raw_env) < 3) {
        hops++;
        _longjmp(raw_env, hops);
    }
    return hops;
}

int main(void) {
    printf("%d\n", attempts(3));
    printf("%d\n", attempts(0));
    printf("%d\n", raw());
    return 0;
}
