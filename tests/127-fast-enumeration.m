// for-in over arrays, sets (sorted first) and dictionaries, with break and
// continue.
// mode: both
#import <Foundation/Foundation.h>
#include <stdio.h>

int main(void) {
    @autoreleasepool {
        NSArray *a = @[ @1, @2, @3, @4, @5, @6 ];
        int s = 0;
        for (NSNumber *n in a) {
            if (n.intValue == 2) continue;
            if (n.intValue == 5) break;
            s += n.intValue;
        }
        printf("%d\n", s);
        NSMutableArray *big = [NSMutableArray array];
        for (int i = 0; i < 100; i++) [big addObject:@(i)];
        long total = 0;
        for (id n in big) total += [n longValue];
        printf("%ld\n", total);
        NSDictionary *d = @{ @"x": @1, @"y": @2 };
        int keys = 0;
        for (NSString *k in d) keys += [d[k] intValue];
        printf("%d\n", keys);
    }
    return 0;
}
