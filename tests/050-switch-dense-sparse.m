// A dense switch (a jump table for clang) and a sparse one, with negative
// and 64-bit cases.
#include <stdio.h>

int dense(int v) {
    switch (v) {
    case 0: return 10; case 1: return 11; case 2: return 12; case 3: return 13;
    case 4: return 14; case 5: return 15; case 6: return 16; case 7: return 17;
    default: return -1;
    }
}

int sparse(long long v) {
    switch (v) {
    case -1000: return 1;
    case 7: return 2;
    case 100000: return 3;
    case 0x100000000ll: return 4;
    default: return 0;
    }
}

int main(void) {
    for (int i = -1; i < 9; i++) printf("%d ", dense(i));
    printf("\n%d %d %d %d %d\n", sparse(-1000), sparse(7), sparse(100000),
           sparse(0x100000000ll), sparse(8));
    return 0;
}
