// @synchronized enters and leaves its lock, including when leaving by
// return and by exception.
// mode: both
#import <Foundation/Foundation.h>
#include <stdio.h>

static int guarded(id lock, int v) {
    @synchronized (lock) {
        if (v < 0) return -1;
        if (v == 0) @throw @"zero";
        return v * 2;
    }
}

int main(void) {
    @autoreleasepool {
        NSObject *lock = [NSObject new];
        printf("%d %d\n", guarded(lock, 4), guarded(lock, -3));
        @try { guarded(lock, 0); } @catch (id e) { printf("threw\n"); }
        @synchronized (lock) { @synchronized (lock) { printf("recursive ok\n"); } }
#if !__has_feature(objc_arc)
        [lock release];
#endif
    }
    return 0;
}
