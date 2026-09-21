// Mixed signed and unsigned: the signed side converts, so -1 > 1u.
#include <stdio.h>

int main(void) {
    int n = -1;
    unsigned u = 1;
    printf("%d\n", n < u);
    long long ll = -1;
    printf("%d\n", ll < u);
    unsigned long ul = 1;
    printf("%d\n", ll < (long long)ul);
    printf("%d\n", n < (long)u);
    return 0;
}
