// ARC: break and continue out of an @autoreleasepool inside a loop pop the
// pool and release the strong locals of the scopes they leave.
// mode: arc
#import <Foundation/Foundation.h>
#include <stdio.h>

@interface Obj : NSObject
@property (nonatomic) int n;
@end
@implementation Obj
- (void)dealloc { printf("dealloc %d\n", _n); }
@end

static void autoreleased(int n) {
    Obj *o = [Obj new];
    o.n = n;
    CFAutorelease((__bridge_retained CFTypeRef)o);
}

int main(void) {
    for (int i = 0; i < 4; i++) {
        @autoreleasepool {
            Obj *local = [Obj new];
            local.n = 100 + i;
            autoreleased(i);
            if (i == 1) continue;
            if (i == 2) break;
            printf("body %d\n", i);
        }
        printf("after pool %d\n", i);
    }
    printf("end\n");
    return 0;
}
