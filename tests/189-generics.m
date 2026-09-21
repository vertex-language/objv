// Lightweight generics and __kindof: type arguments are erased and change
// nothing at run time.
// mode: both
#import <Foundation/Foundation.h>
#include <stdio.h>

@interface Box<__covariant T> : NSObject
@property (nonatomic, strong) T item;
- (T)take;
@end
@implementation Box
- (id)take { return _item; }
@end

int main(void) {
    @autoreleasepool {
        NSArray<NSString *> *words = @[ @"b", @"a", @"c" ];
        NSMutableDictionary<NSString *, NSNumber *> *d = [NSMutableDictionary dictionary];
        for (NSString *w in [words sortedArrayUsingSelector:@selector(compare:)])
            d[w] = @(w.length + [w characterAtIndex:0]);
        printf("%d\n", d[@"c"].intValue);
        Box<NSString *> *b = [Box new];
        b.item = @"boxed";
        printf("%s %lu\n", [b take].UTF8String, (unsigned long)b.item.length);
        __kindof NSString *k = words.firstObject;
        printf("%s\n", [k uppercaseString].UTF8String);
    }
    return 0;
}
