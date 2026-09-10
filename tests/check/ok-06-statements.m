// §7's Objective-C statements.

@interface Flow : NSObject
@property (nonatomic, copy) NSArray<NSString *> *items;
- (void)mayThrow;
@end

@implementation Flow
- (void)mayThrow { }

- (void)enumerate {
    for (NSString *s in self.items) {
        (void)s;
    }
    id element;
    for (element in self.items) { }

    for (NSString *s in self.items) {
        for (int i = 0; i < 3; i++) {
            if (i) continue;
            (void)s;
        }
    }
}

- (void)exceptions {
    @try {
        [self mayThrow];
        @throw [[NSException alloc] init];
    } @catch (NSException *e) {
        (void)e;
        @throw;                 // a rethrow, inside a @catch
    } @catch (id any) {
        (void)any;
    } @finally {
        [self mayThrow];
    }

    @synchronized (self) {
        [self mayThrow];
    }

    @autoreleasepool {
        @autoreleasepool { }
    }
}
@end
