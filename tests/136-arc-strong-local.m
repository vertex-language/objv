// ARC: a strong local releases its object when reassigned and when it goes
// out of scope.
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
    {
        Obj *a = [Obj new];
        a.n = 1;
        printf("reassign\n");
        a = [Obj new];
        a.n = 2;
        printf("leave scope\n");
    }
    printf("after scope\n");
    Obj *b = [Obj new];
    b.n = 3;
    b = nil;
    printf("end\n");
    return 0;
}
