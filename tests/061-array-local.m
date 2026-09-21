// A local array filled and read back.
#include <stdio.h>

int main(void) {
    int sq[10];
    for (int i = 0; i < 10; i++) sq[i] = i * i;
    int s = 0;
    for (int i = 0; i < 10; i++) s += sq[i];
    printf("%d %zu\n", s, sizeof sq);
    return 0;
}
