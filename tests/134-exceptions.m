// @try, @catch by class, @finally, and an exception crossing a frame.
// mode: both
#import <Foundation/Foundation.h>
#include <stdio.h>

static void thrower(int kind) {
    if (kind == 1) @throw [NSException exceptionWithName:@"Boom" reason:@"one" userInfo:nil];
    if (kind == 2) @throw @"a string";
    printf("no throw\n");
}

static void attempt(int kind) {
    @try {
        thrower(kind);
    } @catch (NSException *e) {
        printf("caught %s: %s\n", e.name.UTF8String, e.reason.UTF8String);
    } @catch (id other) {
        printf("caught other %s\n", [other UTF8String]);
    } @finally {
        printf("finally %d\n", kind);
    }
}

int main(void) {
    @autoreleasepool {
        for (int k = 0; k < 3; k++) attempt(k);
        @try {
            @try { thrower(1); }
            @finally { printf("inner finally\n"); }
        } @catch (NSException *e) {
            printf("rethrown to outer\n");
        }
    }
    return 0;
}
