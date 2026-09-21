// Comparing pointers into one array, and null.
#include <stdio.h>
#include <stddef.h>

int main(void) {
    char s[8] = "abcdefg";
    char *lo = s + 1, *hi = s + 5;
    printf("%d %d %d %td\n", lo < hi, lo == hi, hi >= lo, hi - lo);
    char *n = NULL;
    printf("%d %d\n", n == 0, !n);
    ptrdiff_t d = lo - hi;
    printf("%td\n", d);
    return 0;
}
