// A variadic function forwards its va_list to vsnprintf, and a va_list is
// passed to a helper that reads from it.
#include <stdio.h>
#include <stdarg.h>

static char out[128];

static void logf_(const char *fmt, ...) {
    va_list ap;
    va_start(ap, fmt);
    vsnprintf(out, sizeof out, fmt, ap);
    va_end(ap);
    printf("log: %s\n", out);
}

static int take(va_list *ap) { return va_arg(*ap, int); }

static int sum_via_helper(int n, ...) {
    va_list ap;
    va_start(ap, n);
    int s = 0;
    while (n--) s += take(&ap);
    va_end(ap);
    return s;
}

int main(void) {
    logf_("%s=%d (%.2f) %c %lld", "x", 42, 3.14159, 'z', 1ll << 40);
    printf("%d\n", sum_via_helper(4, 10, 20, 30, 40));
    return 0;
}
