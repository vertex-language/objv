// Message sends that do not resolve.

@interface Target : NSObject
- (void)ping;
- (void)take:(NSString *)s;
@end

@implementation Target
- (void)ping { }
- (void)take:(NSString *)s { (void)s; }

- (void)mistakes {
    [self pnig];                    // expect: no instance method 'pnig' on 'Target'
    [Target ping];                  // expect: no class method 'ping' on 'Target'
    [self take:42];                 // expect: passing argument 1 of 'take:'
    int x = 1;
    [x ping];                       // expect: receiver is int
}
@end
