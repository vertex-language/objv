// goto out of two nested loops at once.
#include <stdio.h>

int main(void) {
    int found = -1;
    for (int i = 0; i < 10; i++)
        for (int j = 0; j < 10; j++)
            if (i * j == 42) { found = i * 10 + j; goto done; }
done:
    printf("%d\n", found);
    return 0;
}
