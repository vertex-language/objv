// Foundation put together: model objects to dictionaries, to JSON with
// sorted keys, and parsed back.
// mode: arc
#import <Foundation/Foundation.h>
#include <stdio.h>

@interface Item : NSObject
@property (nonatomic, copy) NSString *name;
@property (nonatomic) NSInteger qty;
@property (nonatomic) double price;
- (NSDictionary *)plist;
@end
@implementation Item
- (NSDictionary *)plist { return @{ @"name": _name, @"qty": @(_qty), @"price": @(_price) }; }
@end

int main(void) {
    @autoreleasepool {
        NSMutableArray *items = [NSMutableArray array];
        NSArray *names = @[ @"pen", @"ink", @"pad" ];
        for (NSUInteger i = 0; i < names.count; i++) {
            Item *it = [Item new];
            it.name = names[i];
            it.qty = (NSInteger)(i + 1) * 2;
            it.price = 0.5 * (double)(i + 1);
            [items addObject:it.plist];
        }
        NSDictionary *root = @{ @"items": items, @"count": @(items.count), @"ok": @YES };
        NSError *err = nil;
        NSData *data = [NSJSONSerialization dataWithJSONObject:root options:NSJSONWritingSortedKeys error:&err];
        NSString *json = [[NSString alloc] initWithData:data encoding:NSUTF8StringEncoding];
        printf("%s\n", json.UTF8String);
        NSDictionary *back = [NSJSONSerialization JSONObjectWithData:data options:0 error:&err];
        double total = 0;
        for (NSDictionary *d in back[@"items"]) total += [d[@"qty"] doubleValue] * [d[@"price"] doubleValue];
        printf("%g %d %d\n", total, [back[@"count"] intValue], err == nil);
    }
    return 0;
}
