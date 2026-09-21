// Two functions calling each other through a forward declaration.
#include <stdio.h>

int is_odd(unsigned n);
int is_even(unsigned n) { return n == 0 ? 1 : is_odd(n - 1); }
int is_odd(unsigned n)  { return n == 0 ? 0 : is_even(n - 1); }

int main(void) {
    printf("%d %d %d\n", is_even(10), is_odd(7), is_even(1001));
    return 0;
}
