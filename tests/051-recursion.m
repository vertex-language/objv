// A function that calls itself.
#include <stdio.h>

int fib(int n) { return n < 2 ? n : fib(n - 1) + fib(n - 2); }
long long fact(int n) { return n <= 1 ? 1 : n * fact(n - 1); }

int main(void) {
    printf("%d %lld\n", fib(20), fact(20));
    return 0;
}
