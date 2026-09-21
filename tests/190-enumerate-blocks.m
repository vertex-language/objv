// Block-based enumeration with index, *stop, and a filtered index set.
// mode: both
#import <Foundation/Foundation.h>
#include <stdio.h>

int main(void) {
    @autoreleasepool {
        NSArray *a = @[ @5, @8, @13, @21, @34 ];
        __block int sum = 0;
        [a enumerateObjectsUsingBlock:^(NSNumber *n, NSUInteger i, BOOL *stop) {
            sum += n.intValue * (int)i;
            if (i == 3) *stop = YES;
        }];
        printf("%d\n", sum);
        NSIndexSet *odd = [a indexesOfObjectsPassingTest:^BOOL(NSNumber *n, NSUInteger i, BOOL *stop) {
            return n.intValue % 2;
        }];
        printf("%lu %lu\n", (unsigned long)odd.count, (unsigned long)odd.firstIndex);
        NSDictionary *d = @{ @"a": @1, @"b": @2 };
        __block int total = 0;
        [d enumerateKeysAndObjectsUsingBlock:^(id k, NSNumber *v, BOOL *stop) { total += v.intValue; }];
        printf("%d\n", total);
        [a enumerateObjectsWithOptions:NSEnumerationReverse usingBlock:^(id n, NSUInteger i, BOOL *stop) {
            printf("%d ", [n intValue]);
        }];
        printf("\n");
    }
    return 0;
}
