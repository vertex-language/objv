// static inline functions, called and taken the address of.
#include <stdio.h>

static inline int sq(int v) { return v * v; }
static inline __attribute__((always_inline)) int cube(int v) { return v * sq(v); }

int main(void) {
    int (*f)(int) = sq;
    printf("%d %d %d\n", sq(7), cube(3), f(12));
    return 0;
}
