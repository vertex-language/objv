// ARC: an object holding a block that captures it strongly is a cycle; a
// weak capture breaks it and the object is deallocated.
// mode: arc
#import <Foundation/Foundation.h>
#include <stdio.h>

@interface Owner : NSObject
@property (nonatomic, copy) void (^action)(void);
@property (nonatomic) int n;
@end
@implementation Owner
- (void)dealloc { printf("dealloc %d\n", _n); }
@end

int main(void) {
    {
        Owner *cyclic = [Owner new];
        cyclic.n = 1;
        cyclic.action = ^{ printf("cyclic %d\n", cyclic.n); };
        cyclic.action();
    }
    printf("cyclic scope left (leaked)\n");
    {
        Owner *o = [Owner new];
        o.n = 2;
        __weak Owner *weakO = o;
        o.action = ^{
            Owner *strong = weakO;
            printf("weak %d\n", strong.n);
        };
        o.action();
    }
    printf("end\n");
    return 0;
}
