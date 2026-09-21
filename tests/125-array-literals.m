// Array literals and subscripting, immutable and mutable.
// mode: both
#import <Foundation/Foundation.h>
#include <stdio.h>

int main(void) {
    @autoreleasepool {
        NSArray *a = @[ @1, @"two", @3.0 ];
        printf("%lu %d %s %g\n", (unsigned long)a.count, [a[0] intValue], [a[1] UTF8String],
               [a[2] doubleValue]);
        NSMutableArray *m = [a mutableCopy];
        m[1] = @2;
        [m addObject:@4];
        m[m.count] = @5;
        for (NSUInteger i = 0; i < m.count; i++) printf("%d ", [m[i] intValue]);
        printf("\n%d\n", [@[] count] == 0);
#if !__has_feature(objc_arc)
        [m release];
#endif
    }
    return 0;
}
