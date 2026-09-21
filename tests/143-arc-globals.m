// ARC: a strong global or static retains what is stored and releases what
// it replaces.
// mode: arc
#import <Foundation/Foundation.h>
#include <stdio.h>

@interface Obj : NSObject
@property (nonatomic) int n;
@end
@implementation Obj
- (void)dealloc { printf("dealloc %d\n", _n); }
@end

Obj *global;
static Obj *file_static;
static Obj *table[2];

static Obj *cached(void) {
    static Obj *once;
    if (!once) { once = [Obj new]; once.n = 30; }
    return once;
}

int main(void) {
    @autoreleasepool {
        global = [Obj new];
        global.n = 10;
        file_static = [Obj new];
        file_static.n = 20;
        printf("%d %d\n", cached().n, cached() == cached());
        table[0] = [Obj new];
        table[0].n = 40;
        table[1] = table[0];
        table[0] = nil;
        printf("table %d\n", table[1].n);
    }
    printf("pool end\n");
    global = [Obj new];
    global.n = 11;
    file_static = nil;
    table[1] = nil;
    printf("%d\n", global.n);
    return 0;
}
