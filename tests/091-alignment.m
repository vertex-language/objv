// _Alignas on locals, globals and members, and _Alignof.
#include <stdio.h>
#include <stdint.h>
#include <stddef.h>

_Alignas(64) static char g[3];
struct s { char c; _Alignas(16) int i; };

int main(void) {
    _Alignas(32) char local[5];
    char other = 0;
    printf("%d %d\n", (int)((uintptr_t)g % 64), (int)((uintptr_t)local % 32));
    printf("%zu %zu %zu\n", _Alignof(struct s), sizeof(struct s), offsetof(struct s, i));
    printf("%zu %zu %zu\n", _Alignof(double), _Alignof(long long), _Alignof(char));
    (void)other;
    return 0;
}
