// A block literal can be variadic: its body starts a va_list like any
// variadic function, and a call passes a var-tail of ints, doubles and a
// struct.
// mode: both
#import <Foundation/Foundation.h>
#include <stdio.h>
#include <stdarg.h>

typedef struct { int a, b; } Pair;

int main(void) {
    int base = 100;
    int (^sum)(int, ...) = ^int(int n, ...) {
        va_list ap;
        va_start(ap, n);
        int s = base;
        for (int i = 0; i < n; i++) s += va_arg(ap, int);
        va_end(ap);
        return s;
    };
    printf("%d %d\n", sum(0), sum(3, 1, 2, 3));
    void (^mixed)(const char *, ...) = ^(const char *kinds, ...) {
        va_list ap;
        va_start(ap, kinds);
        for (const char *k = kinds; *k; k++) {
            if (*k == 'd') printf("d %g\n", va_arg(ap, double));
            if (*k == 'p') { Pair p = va_arg(ap, Pair); printf("p %d %d\n", p.a, p.b); }
        }
        va_end(ap);
    };
    mixed("dpd", 1.5, (Pair){ 3, 4 }, -2.0);
    return 0;
}
