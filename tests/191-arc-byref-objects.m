// ARC: a __block object variable is strong: assigning it inside a block
// retains the new value and releases the old.
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
        __block Obj *current = [Obj new];
        current.n = 1;
        void (^swap)(int) = ^(int n) {
            Obj *o = [Obj new];
            o.n = n;
            current = o;
        };
        swap(2);
        printf("current %d\n", current.n);
        swap(3);
        printf("current %d\n", current.n);
    }
    printf("end\n");
    return 0;
}
