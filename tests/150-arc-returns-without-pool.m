// ARC: +0 returns called with no autorelease pool in place, the way cwindow
// calls windowOf hundreds of times a second. With the return handshake
// nothing reaches a pool and every object is freed as it goes; without it
// each one is autoreleased into no pool at all.
// mode: arc
#import <Foundation/Foundation.h>
#include <stdio.h>

static int live;

@interface Obj : NSObject
@end
@implementation Obj
- (instancetype)init { if ((self = [super init])) live++; return self; }
- (void)dealloc { live--; }
@end

__attribute__((noinline)) Obj *fetch(void) { return [Obj new]; }

static Obj *kept;
__attribute__((noinline)) static Obj *lookup(void) { return kept; }

int main(void) {
    for (int i = 0; i < 1000; i++) {
        Obj *o = fetch();
        (void)o;
    }
    printf("live after fetches: %d\n", live);

    kept = [Obj new];
    for (int i = 0; i < 1000; i++) {
        Obj *o = lookup();
        (void)o;
    }
    kept = nil;
    printf("live after lookups: %d\n", live);
    return 0;
}
