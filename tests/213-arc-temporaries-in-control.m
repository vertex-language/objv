// ARC: objects produced inside a control statement's condition, a for-in's
// collection, a @synchronized operand, the operands of && and ||, nested ?:
// and ?: with no middle, and a for loop's third clause are each released on
// the path that made them, and the collection and the lock live through the
// whole statement. Only what ARC defines is compared: counts once an
// enclosing pool has drained (a +0 result nothing retains is the pool's,
// and whether one is retained at all is the compiler's choice), and that
// the collection and the lock are alive while their statement runs.
// mode: arc
#import <Foundation/Foundation.h>
#include <stdio.h>

static int live;

@interface Obj : NSObject
@property (nonatomic) int n;
+ (instancetype)with:(int)n;
- (NSArray *)items;
@end
@implementation Obj
+ (instancetype)with:(int)n { Obj *o = [self new]; o.n = n; return o; }
- (instancetype)init { if ((self = [super init])) live++; return self; }
- (void)dealloc { live--; }
- (NSArray *)items { return @[ [Obj with:1], [Obj with:2], [Obj with:3] ]; }
@end

static Obj *maybe(int i) { return i % 2 ? [Obj with:i] : nil; }

int main(void) {
    @autoreleasepool {
    int hits = 0;
    for (int i = 0; i < 6; i++) {
        if ([Obj with:i].n > 2) hits++;
        if (maybe(i) && [Obj with:i].n) hits += 10;
        if (maybe(i) || [Obj with:i]) hits += 100;
        switch ([Obj with:i].n % 3) { case 0: hits += 1000; break; default: break; }
    }
    printf("hits %d\n", hits);

    int sum = 0, alive = 1;
    for (Obj *o in [[Obj with:0] items]) {
        alive &= live > 0;
        if (o.n == 2) continue;
        sum += o.n;
    }
    for (Obj *o in [[Obj with:0] items]) {
        if (o.n == 2) break;
    }
    printf("sum %d alive %d\n", sum, alive);

    @synchronized ([Obj with:9]) {
        printf("locked alive %d\n", live > 0);
    }

    int k = 0;
    while ([Obj with:k].n < 3) k++;
    for (int j = 0; j < 3; j = [Obj with:j + 1].n) {}
    Obj *a = [Obj with:1];
    for (int i = 0; i < 4; i++) {
        Obj *x = i == 0 ? [Obj with:5] : i == 1 ? a : (i == 2 ? nil : [Obj with:6]);
        Obj *y = maybe(i) ?: [Obj with:7];
        Obj *z = maybe(i) ?: a;
        (void)x; (void)y; (void)z;
    }
    a = nil;
    printf("k %d\n", k);
    }
    printf("live after pool %d\n", live);
    return 0;
}
