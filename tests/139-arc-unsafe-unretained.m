// ARC: __unsafe_unretained neither retains nor releases; the object's
// lifetime is decided by the strong reference alone.
// mode: arc
#import <Foundation/Foundation.h>
#include <stdio.h>

@interface Obj : NSObject
@property (nonatomic) int n;
@end
@implementation Obj
- (void)dealloc { printf("dealloc %d\n", _n); }
@end

int main(void) {
    Obj *strong = [Obj new];
    strong.n = 5;
    __unsafe_unretained Obj *u = strong;
    printf("%d\n", u.n);
    {
        __unsafe_unretained Obj *u2 = u;
        printf("%d\n", u2.n);
    }
    printf("before clear\n");
    strong = nil;
    printf("after clear\n");
    return 0;
}
