// @finally runs when its @try is left by return, break and continue.
// mode: both
#import <Foundation/Foundation.h>
#include <stdio.h>

static int early(int v) {
    @try {
        if (v) return 1;
        printf("no return\n");
    } @finally {
        printf("finally for %d\n", v);
    }
    return 0;
}

int main(void) {
    @autoreleasepool {
        printf("%d %d\n", early(1), early(0));
        for (int i = 0; i < 4; i++) {
            @try {
                if (i == 1) continue;
                if (i == 3) break;
                printf("body %d\n", i);
            } @finally {
                printf("finally %d\n", i);
            }
        }
    }
    return 0;
}
