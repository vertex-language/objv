// Calling a variadic function through a pointer: the var-tail travels the
// variadic way (on the stack on arm64) and not as named arguments.
#include <stdio.h>
#include <stdarg.h>

static double avg(int n, ...) {
    va_list ap;
    va_start(ap, n);
    double s = 0;
    for (int i = 0; i < n; i++) s += va_arg(ap, double);
    va_end(ap);
    return n ? s / n : 0;
}

typedef struct { long a, b, c; } Big;

static Big pick(int which, ...) {
    va_list ap;
    va_start(ap, which);
    long v = 0;
    for (int i = 0; i <= which; i++) v = va_arg(ap, long);
    va_end(ap);
    return (Big){ v, v * 2, v * 3 };
}

int main(void) {
    int (*pf)(const char *restrict, ...) = printf;
    pf("%s %d %.2f\n", "via pointer", 42, 2.5);
    int (*sp)(char *restrict, size_t, const char *restrict, ...) = snprintf;
    char buf[32];
    sp(buf, sizeof buf, "%d-%d-%d", 1, 2, 3);
    printf("%s\n", buf);
    double (*fa)(int, ...) = avg;
    printf("%g\n", fa(4, 1.0, 2.0, 3.0, 10.0));
    Big (*fp)(int, ...) = pick;
    Big b = fp(2, 10l, 20l, 30l);
    printf("%ld %ld %ld\n", b.a, b.b, b.c);
    return 0;
}
