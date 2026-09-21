// Globals with constant initializers, including addresses of other globals.
#include <stdio.h>

int g = 42;
double d = 2.5;
int arr[4] = { 1, 2, 3 };
int *pg = &g;
int *pmid = &arr[2];
const char *msg = "global";

int main(void) {
    printf("%d %.1f %d %d %d\n", g, d, arr[2], arr[3], *pg);
    *pg = 7;
    printf("%d %d %s\n", g, *pmid, msg);
    return 0;
}
