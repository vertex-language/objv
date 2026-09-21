// A function takes two ints and gives back their sum.
#include <stdio.h>

int add(int s, int x) { return s + x; }

int main(void) {
    printf("%d\n", add(2, 3));
    printf("%d\n", add(-7, 3));
    return 0;
}
