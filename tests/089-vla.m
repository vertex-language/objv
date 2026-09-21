// A variable-length array, sized at run time, in a loop so the stack
// pointer must come back each iteration.
#include <stdio.h>

int sum_to(int n) {
    int v[n];
    for (int i = 0; i < n; i++) v[i] = i + 1;
    int s = 0;
    for (int i = 0; i < n; i++) s += v[i];
    return s + (int)(sizeof v / sizeof v[0]) * 0;
}

int main(void) {
    long total = 0;
    for (int round = 0; round < 10000; round++) total += sum_to(1 + round % 50);
    printf("%ld %zu\n", total, (size_t)sum_to(100));
    int rows = 3, cols = 4;
    double m[rows][cols];
    m[2][3] = 1.25;
    printf("%zu %g\n", sizeof m, m[2][3]);
    return 0;
}
