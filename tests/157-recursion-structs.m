// Recursion that passes and returns structs by value at every level.
#include <stdio.h>

typedef struct { long sum; long count; double mean; } Stats;
typedef struct { int lo, hi; } Range;

Stats walk(Range r) {
    if (r.lo == r.hi) return (Stats){ r.lo, 1, r.lo };
    int mid = (r.lo + r.hi) / 2;
    Stats a = walk((Range){ r.lo, mid });
    Stats b = walk((Range){ mid + 1, r.hi });
    Stats s = { a.sum + b.sum, a.count + b.count, 0 };
    s.mean = (double)s.sum / s.count;
    return s;
}

int main(void) {
    Stats s = walk((Range){ 1, 1000 });
    printf("%ld %ld %g\n", s.sum, s.count, s.mean);
    return 0;
}
