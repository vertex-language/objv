// Enumerators count up from zero or from a given value.
#include <stdio.h>

enum color { RED, GREEN, BLUE };
enum level { LOW = -1, MID = 5, HIGH, TOP = 100 };
typedef enum { A = 1 << 0, B = 1 << 1, C = 1 << 2 } flags;

int main(void) {
    enum color c = BLUE;
    printf("%d %d %d\n", RED, GREEN, c);
    printf("%d %d %d %d\n", LOW, MID, HIGH, TOP);
    flags f = A | C;
    printf("%d %d %zu\n", f, (f & B) != 0, sizeof(enum color));
    return 0;
}
