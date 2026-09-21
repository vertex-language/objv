// Deep recursion with a modest frame: tens of thousands of frames on the
// default stack.
#include <stdio.h>

long depth(long n, long acc) {
    volatile char pad[64];
    pad[0] = (char)n;
    if (n == 0) return acc + pad[0];
    return depth(n - 1, acc + (n & 3)) + 0 * pad[0];
}

int main(void) {
    printf("%ld\n", depth(50000, 0));
    return 0;
}
