// A __block variable is shared by the block and the scope, even after the
// block is copied to the heap.
// mode: both
#import <Foundation/Foundation.h>
#include <stdio.h>

typedef void (^Step)(void);

int main(void) {
    __block int count = 0;
    Step inc = ^{ count++; };
    inc();
    inc();
    printf("%d\n", count);
    Step heap = [inc copy];
    heap();
    count += 10;
    heap();
    printf("%d\n", count);
#if !__has_feature(objc_arc)
    [heap release];
#endif
    return 0;
}
