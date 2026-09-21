// A 16-byte struct by value: two registers.
#include <stdio.h>

struct wide { long long lo; long long hi; };
struct mixed { char c; int i; long long l; };

struct wide bump(struct wide w) { w.lo++; w.hi--; return w; }
long long sum(struct mixed m) { return m.c + m.i + m.l; }

int main(void) {
    struct wide w = bump((struct wide){ 10, 20 });
    printf("%lld %lld\n", w.lo, w.hi);
    printf("%lld\n", sum((struct mixed){ 1, 2, 3000000000ll }));
    return 0;
}
