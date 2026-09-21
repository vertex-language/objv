// NSString basics: length, characters, comparison, substrings.
// mode: both
#import <Foundation/Foundation.h>
#include <stdio.h>

int main(void) {
    @autoreleasepool {
        NSString *s = @"Objective-C";
        printf("%lu %c\n", (unsigned long)s.length, (char)[s characterAtIndex:4]);
        printf("%d %d\n", [s isEqualToString:@"Objective-C"], [s hasPrefix:@"Obj"]);
        printf("%s\n", [s substringFromIndex:10].UTF8String);
        printf("%s\n", [s lowercaseString].UTF8String);
        NSRange r = [s rangeOfString:@"-"];
        printf("%lu\n", (unsigned long)r.location);
        printf("%ld\n", (long)[@"apple" compare:@"banana"]);
    }
    return 0;
}
