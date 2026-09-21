// A block that calls itself through a __block variable.
// mode: both
#import <Foundation/Foundation.h>
#include <stdio.h>

int main(void) {
    __block long (^fib)(int) = nil;
    long (^local)(int) = ^long(int n) { return n < 2 ? n : fib(n - 1) + fib(n - 2); };
    fib = local;
    printf("%ld\n", fib(20));
    __block int depth = 0;
    __block void (^countdown)(int);
    countdown = ^(int n) { depth++; if (n > 0) countdown(n - 1); };
    countdown(10);
    printf("%d\n", depth);
    fib = nil;
    countdown = nil;
    return 0;
}
