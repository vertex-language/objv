// ARC: the +0 return handshake through a class factory method and a getter
// returning an ivar -- each object freed when its last strong reference
// goes, not parked in the pool -- and, with nothing but counts compared, a
// send inside @try, a block returning an object, and a loop with no pool.
// (Where clang's own handshake does not fire -- an invoke's result, a block
// call -- when the object goes is clang's accident, not a rule; only that
// it goes is compared.)
// mode: arc
#import <Foundation/Foundation.h>
#include <stdio.h>

static int live;

@interface Obj : NSObject
@property (nonatomic) int n;
+ (instancetype)objWith:(int)n;
@end
@implementation Obj
+ (instancetype)objWith:(int)n { Obj *o = [self new]; o.n = n; live++; return o; }
- (void)dealloc { live--; if (_n < 100) printf("dealloc %d\n", _n); }
@end

@interface Holder : NSObject
@property (nonatomic, strong) Obj *held;
@end
@implementation Holder
@end

int main(void) {
    @autoreleasepool {
        {
            Obj *a = [Obj objWith:1];
            printf("factory %d\n", a.n);
        }
        printf("after factory scope\n");
        Holder *h = [Holder new];
        h.held = [Obj objWith:2];
        {
            Obj *g = h.held;
            h.held = nil;
            printf("getter %d\n", g.n);
        }
        printf("after getter scope\n");
    }
    @autoreleasepool {
        @try {
            Obj *t = [Obj objWith:103];
            printf("in try %d\n", t.n);
        } @finally {
            printf("finally\n");
        }
        Obj *(^make)(int) = ^Obj *(int n) { return [Obj objWith:n]; };
        Obj *b = make(104);
        printf("block %d\n", b.n);
    }
    printf("live after pool %d\n", live);
    for (int i = 0; i < 500; i++) {
        Obj *o = [Obj objWith:100];
        (void)o;
    }
    printf("live after loop %d\n", live);
    return 0;
}
