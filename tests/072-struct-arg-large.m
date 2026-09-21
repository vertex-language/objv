// A struct over 16 bytes is passed by reference to a copy: the callee's
// changes must not reach the caller.
#include <stdio.h>

struct big { long v[5]; };

long consume(struct big b) {
    long s = 0;
    for (int i = 0; i < 5; i++) { s += b.v[i]; b.v[i] = -1; }
    return s;
}

int main(void) {
    struct big b = { { 1, 2, 3, 4, 5 } };
    printf("%ld\n", consume(b));
    printf("%ld %ld\n", b.v[0], b.v[4]);
    return 0;
}
