// A block with no captures: defined, called, passed, and typedef'd.
// mode: both
#import <Foundation/Foundation.h>
#include <stdio.h>

typedef int (^IntOp)(int, int);

int fold(const int *v, int n, int start, IntOp op) {
    for (int i = 0; i < n; i++) start = op(start, v[i]);
    return start;
}

int main(void) {
    void (^hello)(void) = ^{ printf("hello from a block\n"); };
    hello();
    IntOp add = ^(int a, int b) { return a + b; };
    int v[] = { 1, 2, 3, 4 };
    printf("%d %d\n", fold(v, 4, 0, add), fold(v, 4, 1, ^(int a, int b) { return a * b; }));
    double (^half)(double) = ^double(double x) { return x / 2; };
    printf("%g\n", half(5));
    return 0;
}
