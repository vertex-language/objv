// A table of function pointers, and qsort calling back into the program.
#include <stdio.h>
#include <stdlib.h>

static int inc(int v) { return v + 1; }
static int dbl(int v) { return v * 2; }
static int neg(int v) { return -v; }
static int (*const pipeline[])(int) = { inc, dbl, neg, dbl };

static int by_desc(const void *a, const void *b) {
    return *(const int *)b - *(const int *)a;
}

int main(void) {
    int v = 5;
    for (unsigned i = 0; i < sizeof pipeline / sizeof *pipeline; i++) v = pipeline[i](v);
    printf("%d\n", v);
    int a[] = { 4, 9, 1, 7, 3 };
    qsort(a, 5, sizeof a[0], by_desc);
    for (int i = 0; i < 5; i++) printf("%d ", a[i]);
    printf("\n");
    return 0;
}
