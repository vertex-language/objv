// The conditional operator with struct, pointer, and mixed arithmetic arms.
#include <stdio.h>

typedef struct { int a, b, c, d, e; } Five;

int main(void) {
    Five x = { 1, 2, 3, 4, 5 }, y = { 10, 20, 30, 40, 50 };
    for (int i = 0; i < 2; i++) {
        Five z = i ? x : y;
        printf("%d %d\n", z.a, z.e);
    }
    int arr[3] = { 7, 8, 9 };
    int *p = 0;
    int *q = p ? p : arr + 1;
    printf("%d\n", *q);
    int k = 3;
    double d = k > 2 ? 1 : 2.5;
    unsigned u = k > 5 ? 1u : -1;
    printf("%g %u\n", d, u);
    const char *s = k & 1 ? "odd" : "even";
    printf("%s\n", s);
    return 0;
}
