// A message to nil does nothing and returns zero, of every result type.
// mode: both
#import <Foundation/Foundation.h>
#include <stdio.h>

typedef struct { long a, b, c, d; } Big;
typedef struct { float x, y; } Small;

@interface Thing : NSObject
- (int)i;
- (double)d;
- (id)o;
- (Big)big;
- (Small)small;
- (long long)ll;
@end
@implementation Thing
- (int)i { return 1; }
- (double)d { return 1; }
- (id)o { return self; }
- (Big)big { return (Big){ 1, 2, 3, 4 }; }
- (Small)small { return (Small){ 1, 2 }; }
- (long long)ll { return 1; }
@end

int main(void) {
    Thing *t = nil;
    Big b = [t big];
    Small s = [t small];
    printf("%d %g %d %lld\n", [t i], [t d], [t o] == nil, [t ll]);
    printf("%ld %ld %g %g\n", b.a, b.d, s.x, s.y);
    return 0;
}
