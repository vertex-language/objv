// Key-value coding: properties read and written by name, boxed and
// unboxed, and a key path.
// mode: both
#import <Foundation/Foundation.h>
#include <stdio.h>

@interface Inner : NSObject
@property (nonatomic) double weight;
@end
@implementation Inner
@end

@interface Outer : NSObject
@property (nonatomic, copy) NSString *name;
@property (nonatomic) int count;
@property (nonatomic, strong) Inner *inner;
@end
@implementation Outer
@end

int main(void) {
    @autoreleasepool {
        Outer *o = [Outer new];
        o.inner = [Inner new];
        [o setValue:@"kv" forKey:@"name"];
        [o setValue:@12 forKey:@"count"];
        [o setValue:@2.5 forKeyPath:@"inner.weight"];
        printf("%s %d %g\n", o.name.UTF8String, o.count, o.inner.weight);
        printf("%s %d\n", [[o valueForKey:@"name"] UTF8String], [[o valueForKey:@"count"] intValue]);
        NSArray *arr = @[ @3, @9, @4 ];
        printf("%d %d\n", [[arr valueForKeyPath:@"@max.intValue"] intValue],
               [[arr valueForKeyPath:@"@sum.intValue"] intValue]);
    }
    return 0;
}
