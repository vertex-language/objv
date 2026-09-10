// A class, its superclass, its protocols, and its methods.

@interface Base : NSObject
- (void)ping;
+ (instancetype)shared;
@end

@implementation Base
- (void)ping { }
+ (instancetype)shared { return [[self alloc] init]; }
@end

@interface Derived : Base <NSCopying>
- (void)pong;
@end

@implementation Derived
- (void)pong { [self ping]; }
- (id)copyWithZone:(void *)zone { (void)zone; return self; }

- (void)uses {
    Base *b = [Derived shared];   // instancetype follows the receiver
    Derived *d = [Derived shared];
    [b ping];
    [d pong];
    [super ping];
    id any = d;
    [any pong];
    Class k = [d class];
    (void)b; (void)k;
}
@end
