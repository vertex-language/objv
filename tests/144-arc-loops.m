// ARC: a strong local declared in a loop body is released each iteration,
// including when the iteration ends by break or continue.
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
    for (int i = 0; i < 5; i++) {
        Obj *o = [Obj new];
        o.n = i;
        if (i == 1) continue;
        if (i == 3) break;
        printf("body %d\n", i);
    }
    printf("after for\n");
    int j = 10;
    while (1) {
        Obj *o = [Obj new];
        o.n = j;
        if (++j == 12) break;
    }
    printf("end\n");
    return 0;
}
