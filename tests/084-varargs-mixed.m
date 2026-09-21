// Variadic doubles, long longs, pointers and structs, and va_copy.
#include <stdio.h>
#include <stdarg.h>

struct pt { int x, y; };

double avg(const char *fmt, ...) {
    va_list ap, again;
    va_start(ap, fmt);
    va_copy(again, ap);
    double s = 0;
    int n = 0;
    for (const char *f = fmt; *f; f++, n++) {
        switch (*f) {
        case 'd': s += va_arg(ap, double); break;
        case 'l': s += va_arg(ap, long long); break;
        case 'i': s += va_arg(ap, int); break;
        case 'p': { struct pt p = va_arg(ap, struct pt); s += p.x + p.y; break; }
        }
    }
    va_end(ap);
    printf("first again: %g\n", va_arg(again, double));
    va_end(again);
    return s / n;
}

int main(void) {
    printf("%g\n", avg("dlip", 1.5, 10000000000ll, 3, (struct pt){ 4, 5 }));
    char c = 'A'; float f = 2.5f;
    printf("%c %g\n", c, f);
    return 0;
}
