// size_t, ssize_t, uintptr_t and ptrdiff_t: 64-bit on this target, and
// wrapping below zero for the unsigned ones.
#include <stdio.h>
#include <stddef.h>
#include <stdint.h>
#include <sys/types.h>

size_t sub(size_t a, size_t b) { return a - b; }

int main(void) {
    printf("%zu %zu\n", sizeof(size_t), sub(3, 5));
    ssize_t s = -5;
    printf("%zd %zu\n", s * 2, (size_t)s + 6);
    uintptr_t u = (uintptr_t)-1;
    printf("%lu\n", (unsigned long)(u >> 60));
    size_t big = (size_t)1 << 40;
    printf("%zu %zu\n", big / 1000, big % 1000);
    return 0;
}
