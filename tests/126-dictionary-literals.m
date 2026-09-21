// Dictionary literals and keyed subscripting, printed in sorted key order.
// mode: both
#import <Foundation/Foundation.h>
#include <stdio.h>

int main(void) {
    @autoreleasepool {
        NSDictionary *d = @{ @"b": @2, @"a": @1, @"c": @"three" };
        NSMutableDictionary *m = [d mutableCopy];
        m[@"d"] = @4;
        [m removeObjectForKey:@"b"];
        for (NSString *k in [m.allKeys sortedArrayUsingSelector:@selector(compare:)])
            printf("%s=%s\n", k.UTF8String, [m[k] description].UTF8String);
        printf("%d %lu\n", d[@"zz"] == nil, (unsigned long)d.count);
#if !__has_feature(objc_arc)
        [m release];
#endif
    }
    return 0;
}
