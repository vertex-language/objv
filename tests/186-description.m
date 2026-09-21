// Overriding -description changes what %@ prints, in a string, an array
// and a dictionary.
// mode: both
#import <Foundation/Foundation.h>
#include <stdio.h>

@interface Money : NSObject
@property (nonatomic) long cents;
@end
@implementation Money
- (NSString *)description { return [NSString stringWithFormat:@"$%ld.%02ld", _cents / 100, _cents % 100]; }
@end

int main(void) {
    @autoreleasepool {
        Money *m = [Money new];
        m.cents = 12345;
        printf("%s\n", [NSString stringWithFormat:@"price %@", m].UTF8String);
        Money *n = [Money new];
        n.cents = 7;
        NSArray *a = @[ m, n ];
        printf("%s\n", [[a componentsJoinedByString:@", "] UTF8String]);
        printf("%s\n", [@[ @1, @"x" ] description].UTF8String);
    }
    return 0;
}
