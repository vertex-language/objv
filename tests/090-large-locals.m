// A frame larger than any single immediate offset, with locals on both
// sides of the big array.
#include <stdio.h>
#include <string.h>

int deep(int seed) {
    int before = seed;
    char big[70000];
    int after = seed * 2;
    memset(big, 0, sizeof big);
    big[0] = 1;
    big[sizeof big - 1] = 2;
    big[40000] = 3;
    return before + after + big[0] + big[sizeof big - 1] + big[40000];
}

int main(void) {
    printf("%d\n", deep(10));
    printf("%d\n", deep(deep(1)));
    return 0;
}
