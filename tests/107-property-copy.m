// A copy property stores a copy: a later change to the mutable original is
// not seen.
// mode: both
#import <Foundation/Foundation.h>
#include <stdio.h>

@interface Named : NSObject
@property (nonatomic, copy) NSString *name;
@property (nonatomic, strong) NSString *shared;
@end
@implementation Named
@end

int main(void) {
    @autoreleasepool {
        NSMutableString *m = [NSMutableString stringWithString:@"first"];
        Named *n = [Named new];
        n.name = m;
        n.shared = m;
        [m appendString:@"-changed"];
        printf("%s | %s\n", n.name.UTF8String, n.shared.UTF8String);
        printf("%d %d\n", n.name == (id)m, n.shared == (id)m);
    }
    return 0;
}
