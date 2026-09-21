// NSNumber literals and boxed expressions.
// mode: both
#import <Foundation/Foundation.h>
#include <stdio.h>

int main(void) {
    @autoreleasepool {
        NSNumber *i = @42, *d = @2.5, *b = @YES, *c = @'z', *u = @7u, *l = @(1ll << 40);
        int x = 5;
        NSNumber *e = @(x * 3);
        printf("%d %g %d %c %u %lld %d\n", i.intValue, d.doubleValue, b.boolValue, c.charValue,
               u.unsignedIntValue, l.longLongValue, e.intValue);
        printf("%s %s\n", @(3.0f).stringValue.UTF8String, @(-1).stringValue.UTF8String);
        NSString *boxed = @("boxed");
        printf("%s\n", boxed.UTF8String);
    }
    return 0;
}
