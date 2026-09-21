// A while loop, including one whose body never runs.
#include <stdio.h>

int main(void) {
    int n = 27, steps = 0;
    while (n != 1) {
        n = n % 2 ? 3 * n + 1 : n / 2;
        steps++;
    }
    printf("%d\n", steps);
    while (0) printf("never\n");
    return 0;
}
