// Blocks stored in a collection, called later, each with its own captures.
// mode: both
#import <Foundation/Foundation.h>
#include <stdio.h>

typedef int (^Adder)(int);

int main(void) {
    @autoreleasepool {
        NSMutableArray *adders = [NSMutableArray array];
        for (int i = 1; i <= 3; i++) {
            Adder a = ^(int v) { return v + i * 100; };
            [adders addObject:[a copy]];
        }
        for (Adder a in adders) printf("%d\n", a(5));
        NSArray *sorted = [@[ @3, @1, @2 ] sortedArrayUsingComparator:^NSComparisonResult(id x, id y) {
            return [y compare:x];
        }];
        for (NSNumber *n in sorted) printf("%d ", n.intValue);
        printf("\n");
    }
    return 0;
}
