// §6.9 Blocks, and §5.7's block pointer declarator

@class NSString;

typedef void (^VoidBlock)(void);
typedef int (^IntBlock)(int);

@interface Blocks
- (void)perform:(void (^)(void))block;
- (void)completion:(void (^)(id result, id error))handler;
@end

@implementation Blocks
- (void)perform:(void (^)(void))block { block(); }
- (void)completion:(void (^)(id, id))handler { handler(nil, nil); }

- (void)literals {
    // No parameter list at all, which is not the same text as ( void )
    VoidBlock simple = ^{ };
    VoidBlock explicitVoid = ^(void){ };

    // Parameters, and an explicit return type
    IntBlock doubler = ^(int x) { return x * 2; };
    IntBlock typed = ^int(int x) { return x; };

    // A block that captures, and __block, which makes the capture mutable
    __block int counter = 0;
    int captured = 1;
    VoidBlock counting = ^{ counter += captured; };

    // Blocks nest, and a block may return a block
    VoidBlock outer = ^{
        VoidBlock inner = ^{ counter++; };
        inner();
    };
    IntBlock (^makeBlock)(void) = ^{ return doubler; };

    // Invocation, and a block as a message argument
    doubler(2);
    [self perform:^{ counter = 0; }];
    [self completion:^(id result, id error) { (void)result; (void)error; }];

    // The weak/strong dance, which is what __typeof__ is for
    __weak __typeof__(self) weakSelf = self;
    VoidBlock retainCycleFree = ^{
        __strong __typeof__(weakSelf) strongSelf = weakSelf;
        [strongSelf perform:^{ }];
    };

    (void)simple; (void)explicitVoid; (void)typed; (void)counting;
    (void)outer; (void)makeBlock; (void)retainCycleFree;
}
@end
