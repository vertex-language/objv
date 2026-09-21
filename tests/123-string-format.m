// stringWithFormat: with %@, ints, doubles and C strings.
// mode: both
#import <Foundation/Foundation.h>
#include <stdio.h>

int main(void) {
    @autoreleasepool {
        NSString *who = @"objv";
        NSString *s = [NSString stringWithFormat:@"%@ has %d tests, %.2f%% %s", who, 150, 97.5, "done"];
        printf("%s\n", s.UTF8String);
        NSMutableString *m = [NSMutableString string];
        for (int i = 0; i < 3; i++) [m appendFormat:@"[%d]", i];
        printf("%s\n", m.UTF8String);
        printf("%d\n", [@"42" intValue] + [@"0.5" doubleValue] > 42);
    }
    return 0;
}
