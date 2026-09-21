// Loop conditions with side effects and short-circuits, evaluated exactly
// once per iteration.
#include <stdio.h>

static int calls;
static int next(void) { return ++calls; }

int main(void) {
    int n;
    while ((n = next()) < 5 && n != 3) printf("n=%d\n", n);
    printf("calls %d\n", calls);
    int a[] = { 3, 1, 4, 1, 5, 9, 2, 6 };
    int i = 0;
    while (i < 8 && a[i] != 9) i++;
    printf("found at %d\n", i);
    const char *s = "abc";
    int len = 0;
    while (*s++) len++;
    printf("%d\n", len);
    return 0;
}
