// sizeof of types and expressions, including expressions it does not
// evaluate.
#include <stdio.h>

int main(void) {
    int i = 0;
    size_t s = sizeof(i++);
    printf("%zu %d\n", s, i);
    printf("%zu %zu %zu %zu\n", sizeof(char), sizeof(short), sizeof(long), sizeof(void *));
    printf("%zu %zu\n", sizeof(int[7]), sizeof("four"));
    double d[3];
    printf("%zu %zu\n", sizeof d, sizeof d[0]);
    printf("%zu\n", sizeof(int (*)(void)));
    return 0;
}
