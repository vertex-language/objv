// ARC: a __block object variable is owned by its structure -- released
// where its scope ends, every iteration of a loop that declares it; moved
// to the heap when a block capturing it is copied, and kept alive there by
// the copy after the scope that declared it is gone; and reassigned through
// that copy. __block counters declared in a loop are a new variable each
// time round.
// mode: arc
#import <Foundation/Foundation.h>
#include <stdio.h>

static int live;

@interface Obj : NSObject
@property (nonatomic) int n;
+ (instancetype)newWith:(int)n;
@end
@implementation Obj
+ (instancetype)newWith:(int)n { Obj *o = [self new]; o.n = n; return o; }
- (instancetype)init { if ((self = [super init])) live++; return self; }
- (void)dealloc { live--; printf("dealloc %d\n", _n); }
@end

int main(void) {
    for (int i = 0; i < 3; i++) {
        __block Obj *each = [Obj newWith:i];
        void (^touch)(void) = ^{ each.n += 10; };
        touch();
        printf("iteration %d -> %d\n", i, each.n);
    }
    printf("after loop live %d\n", live);

    int (^kept)(int);
    {
        __block Obj *held = [Obj newWith:100];
        kept = [^int(int next) {
            int old = held.n;
            if (next) held = [Obj newWith:next];
            return old;
        } copy];
    }
    printf("kept %d\n", kept(200));
    printf("kept %d\n", kept(0));
    kept = nil;
    printf("after kept live %d\n", live);

    @autoreleasepool {
        NSMutableArray *counters = [NSMutableArray array];
        for (int i = 0; i < 3; i++) {
            __block int count = i * 100;
            [counters addObject:[^int { return ++count; } copy]];
        }
        for (int (^c)(void) in counters) printf("%d ", c());
        for (int (^c)(void) in counters) printf("%d ", c());
        printf("\n");
    }
    printf("end live %d\n", live);
    return 0;
}
