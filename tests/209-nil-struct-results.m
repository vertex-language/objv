// A message to nil yields a zero struct even when the struct comes back
// through memory, including memory a live receiver filled on the loop's
// previous trip, a property read with dot syntax, and a send through id.
// mode: both
#import <Foundation/Foundation.h>
#include <stdio.h>

typedef struct { long a, b, c, d, e; } Big;

@interface Src : NSObject
@property (nonatomic) Big big;
- (Big)bigWith:(long)v;
@end
@implementation Src
- (Big)bigWith:(long)v { return (Big){ v, v, v, v, v }; }
@end

int main(void) {
    Src *live = [Src new];
    live.big = (Big){ 1, 2, 3, 4, 5 };
    Src *targets[] = { live, nil, live, nil };
    for (int i = 0; i < 4; i++) {
        Big b = [targets[i] bigWith:i + 10];
        Big p = targets[i].big;
        printf("%ld %ld | %ld %ld\n", b.a, b.e, p.a, p.e);
    }
    id anything = nil;
    Big z = [anything bigWith:9];
    printf("%ld %ld\n", z.a, z.e);
    NSRect r = [(NSValue *)nil rectValue];
    printf("%g %g\n", r.origin.x, r.size.height);
    return 0;
}
