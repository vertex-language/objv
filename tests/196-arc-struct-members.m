// ARC: a C struct with a __strong member is non-trivial: copying it
// retains, destroying it releases.
// mode: arc
#import <Foundation/Foundation.h>
#include <stdio.h>

@interface Obj : NSObject
@property (nonatomic) int n;
@end
@implementation Obj
- (void)dealloc { printf("dealloc %d\n", _n); }
@end

typedef struct { int tag; Obj *obj; } Slot;

static Slot make(int n) { Obj *o = [Obj new]; o.n = n; return (Slot){ n, o }; }

int main(void) {
    {
        Slot a = make(1);
        Slot b = a;
        a.obj = nil;
        printf("b holds %d\n", b.obj.n);
        b = make(2);
        printf("b now %d\n", b.obj.n);
    }
    printf("end\n");
    return 0;
}
