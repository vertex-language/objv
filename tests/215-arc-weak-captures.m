// ARC: a __weak variable captured by a block is a weak reference in the
// block too: the weak-self pattern in a method breaks the cycle, a copied
// block that outlives its object reads nil, a nested block captures through
// the outer one, and a stack block's weak field is unregistered when its
// scope ends, before the object dies.
// mode: arc
#import <Foundation/Foundation.h>
#include <stdio.h>

@interface Owner : NSObject
@property (nonatomic, copy) int (^action)(void);
@property (nonatomic) int n;
- (void)arm;
@end
@implementation Owner
- (void)arm {
    __weak Owner *weakSelf = self;
    self.action = ^int {
        Owner *strongSelf = weakSelf;
        return strongSelf ? strongSelf.n : -1;
    };
}
- (void)dealloc { printf("dealloc %d\n", _n); }
@end

int main(void) {
    int (^outlives)(void);
    {
        Owner *o = [Owner new];
        o.n = 1;
        [o arm];
        printf("action %d\n", o.action());
        outlives = o.action;
    }
    printf("after owner scope, action %d\n", outlives());
    outlives = nil;

    int (^nested)(void);
    {
        Owner *p = [Owner new];
        p.n = 2;
        __weak Owner *w = p;
        nested = ^int {
            int (^inner)(void) = ^int { return w ? w.n : -2; };
            return inner();
        };
        printf("nested %d\n", nested());
    }
    printf("nested after %d\n", nested());
    nested = nil;

    Owner *q = [Owner new];
    q.n = 3;
    {
        __weak Owner *w = q;
        int (^stack)(void) = ^int { return w.n; };
        printf("stack %d\n", stack());
    }
    q = nil;
    printf("end\n");
    return 0;
}
