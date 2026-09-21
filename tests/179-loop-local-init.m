// A local with an initializer inside a loop is initialized afresh every
// iteration, arrays and structs included.
#include <stdio.h>

struct acc { int n; int seen[4]; };

int main(void) {
    for (int round = 0; round < 3; round++) {
        int count = 0;
        int buf[4] = { 0 };
        struct acc a = { 0 };
        for (int i = 0; i <= round; i++) { count++; buf[i]++; a.seen[i] = i + 1; a.n++; }
        printf("%d %d %d %d | %d %d %d\n", count, buf[0], buf[1], buf[3], a.n, a.seen[0], a.seen[3]);
    }
    return 0;
}
