// Right shift of a negative int is arithmetic on this target: the sign bit
// is copied in.
#include <stdio.h>

int shr(int x, int n) { return x >> n; }

int main(void) {
    printf("%d %d %d\n", shr(-16, 2), shr(-1, 31), shr(1024, 3));
    long long v = -(1ll << 40);
    printf("%lld\n", v >> 20);
    return 0;
}
