// §6.8's object literals, and §6.2's subscripting.

@interface Literals : NSObject
@end

@implementation Literals
- (void)build {
    id s = @"string";
    id n = @42;
    id f = @3.5;
    id c = @'c';
    id b = @__objc_yes;
    id e = @(1 + 2);
    NSArray<NSString *> *arr = @[@"a", @"b"];
    NSDictionary<NSString *, NSNumber *> *dict = @{@"k": @1};

    NSString *first = arr[0];
    NSNumber *value = dict[@"k"];

    NSMutableArray<NSString *> *marr;
    marr[0] = @"x";
    NSMutableDictionary<NSString *, NSNumber *> *mdict;
    mdict[@"k"] = @2;

    (void)s; (void)n; (void)f; (void)c; (void)b; (void)e;
    (void)first; (void)value;
}
@end
