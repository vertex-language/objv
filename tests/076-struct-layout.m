// Padding and alignment: sizeof and offsetof as clang lays them out.
#include <stdio.h>
#include <stddef.h>

struct a { char c; int i; };
struct b { char c; double d; char e; };
struct c { short s; char c; };
struct d { char c[3]; long long l; short s; };

int main(void) {
    printf("%zu %zu\n", sizeof(struct a), offsetof(struct a, i));
    printf("%zu %zu %zu\n", sizeof(struct b), offsetof(struct b, d), offsetof(struct b, e));
    printf("%zu %zu\n", sizeof(struct c), _Alignof(struct c));
    printf("%zu %zu %zu\n", sizeof(struct d), offsetof(struct d, l), offsetof(struct d, s));
    return 0;
}
