// ARC: locals are released at the end of their scope, in reverse order of
// declaration, and an object two locals share lives until both are gone.
// mode: arc
#import <Foundation/Foundation.h>
#include <stdio.h>

@interface Obj : NSObject
@property (nonatomic) int n;
@end
@implementation Obj
- (void)dealloc { printf("dealloc %d\n", _n); }
@end

static Obj *make(int n) { Obj *o = [Obj alloc]; o = [o init]; o.n = n; return o; }

int main(void) {
    @autoreleasepool {
        {
            Obj *a = make(1);
            Obj *b = make(2);
            Obj *c = make(3);
            Obj *shared = b;
            (void)a; (void)c;
            {
                Obj *d = make(4);
                (void)d;
                printf("inner end\n");
            }
            b = nil;
            printf("b cleared, shared %d\n", shared.n);
        }
        printf("outer end\n");
    }
    return 0;
}
