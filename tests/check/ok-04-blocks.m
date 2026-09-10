// §6.9's blocks, and §5.7's block pointer.

@interface Blocks : NSObject
- (void)perform:(void (^)(void))block;
@end

@implementation Blocks
- (void)perform:(void (^)(void))block { block(); }

- (void)use {
    void (^simple)(void) = ^{ };
    int (^doubler)(int) = ^(int x) { return x * 2; };
    int (^typed)(int) = ^int(int x) { return x; };

    __block int counter = 0;
    void (^counting)(void) = ^{ counter++; };

    simple();
    counting();
    int n = doubler(2) + typed(3);

    [self perform:^{ counter = 0; }];

    __weak __typeof__(self) weakSelf = self;
    [self perform:^{
        __strong __typeof__(weakSelf) strongSelf = weakSelf;
        [strongSelf perform:^{ }];
    }];

    if (simple) { }             // a block pointer is scalar
    (void)n;
}
@end
