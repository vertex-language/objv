// ARC: new/alloc/copy/init-family methods return +1, everything else +0;
// either way every object is freed by the time the pool ends. (When a +0
// object is freed is 141's question; this one only counts.)
// mode: arc
#import <Foundation/Foundation.h>
#include <stdio.h>

static int live;

@interface Obj : NSObject <NSCopying>
@property (nonatomic) int n;
+ (instancetype)newWithN:(int)n;
+ (instancetype)objWithN:(int)n;
@end
@implementation Obj
- (instancetype)init { if ((self = [super init])) live++; return self; }
+ (instancetype)newWithN:(int)n { Obj *o = [self new]; o.n = n; return o; }
+ (instancetype)objWithN:(int)n { Obj *o = [self new]; o.n = n; return o; }
- (id)copyWithZone:(NSZone *)zone { return [Obj newWithN:_n + 100]; }
- (void)dealloc { live--; }
@end

int main(void) {
    @autoreleasepool {
        Obj *a = [Obj newWithN:1];
        Obj *b = [Obj objWithN:2];
        Obj *c = [a copy];
        Obj *d = [[Obj alloc] init];
        printf("%d %d %d %d live %d\n", a.n, b.n, c.n, d.n, live);
    }
    printf("live after pool %d\n", live);
    return 0;
}
