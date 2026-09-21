// Division and remainder by constants (clang turns these into multiplies),
// across signs and the whole range.
#include <stdio.h>
#include <stdint.h>

int main(void) {
    int32_t v[] = { 0, 1, 6, 7, -7, -8, 100, -100, INT32_MAX, INT32_MIN };
    for (int i = 0; i < 10; i++)
        printf("%d %d %d %d %d\n", v[i] / 3, v[i] % 3, v[i] / 7, v[i] / -5, v[i] / 16);
    uint32_t u[] = { 0, 9, 1000000007u, UINT32_MAX };
    for (int i = 0; i < 4; i++) printf("%u %u %u\n", u[i] / 3, u[i] % 10, u[i] / 1000);
    int64_t w = -123456789012345ll;
    printf("%lld %lld\n", (long long)(w / 10), (long long)(w % 10));
    return 0;
}
