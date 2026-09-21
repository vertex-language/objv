// A block captures a variable's value at the point it is made.
// mode: both
#import <Foundation/Foundation.h>
#include <stdio.h>

typedef struct { int a; double b; } Pair;

int main(void) {
    int x = 10;
    Pair p = { 1, 2.5 };
    const char *s = "captured";
    int (^get)(void) = ^{ return x; };
    void (^show)(void) = ^{ printf("%d %g %s\n", p.a, p.b, s); };
    x = 20;
    p.a = 99;
    printf("%d %d\n", get(), x);
    show();
    return 0;
}
