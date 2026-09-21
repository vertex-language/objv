// ARC: a +0 return from a C function. clang pairs the callee's
// objc_autoreleaseReturnValue with the caller's
// objc_retainAutoreleasedReturnValue (and the mov x29, x29 marker) so the
// object never reaches the pool: it is released at the end of the caller's
// scope, not when the pool drains.
// mode: arc
#import <Foundation/Foundation.h>
#include <stdio.h>

@interface Obj : NSObject
@property (nonatomic) int n;
@end
@implementation Obj
- (void)dealloc { printf("dealloc %d\n", _n); }
@end

__attribute__((noinline)) Obj *make(int n) {
    Obj *o = [Obj new];
    o.n = n;
    return o;
}

int main(void) {
    @autoreleasepool {
        {
            Obj *o = make(1);
            printf("got %d\n", o.n);
        }
        printf("scope end\n");
    }
    printf("pool end\n");
    return 0;
}
