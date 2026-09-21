// Struct results from messages: in registers, HFA, and through memory.
// mode: both
#import <Foundation/Foundation.h>
#include <stdio.h>

typedef struct { int a, b; } Pair;
typedef struct { double x, y, w, h; } Rect4;
typedef struct { long v[5]; } Big;

@interface Maker : NSObject
- (Pair)pair:(int)v;
- (Rect4)rect;
- (Big)big:(long)base;
- (NSRange)range;
@end
@implementation Maker
- (Pair)pair:(int)v { return (Pair){ v, -v }; }
- (Rect4)rect { return (Rect4){ 1, 2, 3, 4 }; }
- (Big)big:(long)base { Big b; for (int i = 0; i < 5; i++) b.v[i] = base + i; return b; }
- (NSRange)range { return NSMakeRange(5, 10); }
@end

int main(void) {
    Maker *m = [Maker new];
    Pair p = [m pair:3];
    Rect4 r = [m rect];
    Big b = [m big:100];
    NSRange rg = [m range];
    printf("%d %d | %g %g %g %g | %ld %ld | %lu %lu\n", p.a, p.b, r.x, r.y, r.w, r.h,
           b.v[0], b.v[4], (unsigned long)rg.location, (unsigned long)rg.length);
    return 0;
}
