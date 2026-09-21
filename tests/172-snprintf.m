// snprintf into a buffer: widths, precision, flags, and truncation.
#include <stdio.h>

int main(void) {
    char b[64];
    int n = snprintf(b, sizeof b, "[%5d|%-5d|%05d|%+d|%x|%#o]", 42, 42, 42, 42, 255, 8);
    printf("%s %d\n", b, n);
    snprintf(b, sizeof b, "[%8.3f|%-8.2e|%g|%.0f]", 3.14159, 12345.678, 0.0001, 2.5);
    printf("%s\n", b);
    char small[6];
    n = snprintf(small, sizeof small, "%s", "truncated");
    printf("%s %d\n", small, n);
    snprintf(b, sizeof b, "%.3s|%10s|%c%%", "abcdef", "right", 'q');
    printf("%s\n", b);
    return 0;
}
