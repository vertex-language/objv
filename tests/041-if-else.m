// if, else if, else.
#include <stdio.h>

const char *sign(int v) {
    if (v < 0) return "negative";
    else if (v == 0) return "zero";
    else return "positive";
}

int main(void) {
    printf("%s %s %s\n", sign(-4), sign(0), sign(9));
    return 0;
}
