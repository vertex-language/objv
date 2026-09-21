// goto backward, as a loop.
#include <stdio.h>

int main(void) {
    int i = 0;
again:
    printf("%d\n", i);
    if (++i < 3) goto again;
    return 0;
}
