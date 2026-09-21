// ARC: a parameter is +0 to the callee; storing it retains, and the callee
// never releases what it was lent.
// mode: arc
#import <Foundation/Foundation.h>
#include <stdio.h>

@interface Obj : NSObject
@property (nonatomic) int n;
@end
@implementation Obj
- (void)dealloc { printf("dealloc %d\n", _n); }
@end

static Obj *kept;

static void look(Obj *o) { printf("look %d\n", o.n); }
static void keep(Obj *o) { kept = o; }
static void replace(Obj *o) { o = [Obj new]; o.n = 99; printf("replaced locally\n"); }
static void out(Obj * __autoreleasing *p) { *p = [Obj new]; (*p).n = 3; }

int main(void) {
    @autoreleasepool {
        Obj *a = [Obj new];
        a.n = 1;
        look(a);
        keep(a);
        replace(a);
        a = nil;
        printf("a cleared, kept %d\n", kept.n);
        kept = nil;
        Obj *o;
        out(&o);
        printf("out %d\n", o.n);
    }
    printf("end\n");
    return 0;
}
