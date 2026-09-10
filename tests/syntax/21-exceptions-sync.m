// §7.2 Exception Handling, §7.3 Synchronization and Autorelease Pools

@class NSException, NSError;

@interface Thrower
- (void)mayThrow;
@end

@implementation Thrower
- (void)mayThrow { }

- (void)exceptions {
    // @try with one @catch
    @try {
        [self mayThrow];
    } @catch (NSException *e) {
        (void)e;
    }

    // Several @catch clauses, most specific first, and a @finally
    @try {
        [self mayThrow];
    } @catch (NSError *error) {
        (void)error;
    } @catch (NSException *e) {
        (void)e;
    } @catch (id anything) {
        (void)anything;
    } @finally {
        [self mayThrow];
    }

    // The catch-everything clause, which names nothing
    @try {
        [self mayThrow];
    } @catch (...) {
    }

    // A @try with only a @finally
    @try {
        [self mayThrow];
    } @finally {
    }

    // @throw, and the bare form that rethrows inside a @catch
    @try {
        @throw (id)0;
    } @catch (id e) {
        (void)e;
        @throw;
    }

    // They nest
    @try {
        @try {
            @throw (id)0;
        } @finally {
        }
    } @catch (id e) {
        (void)e;
    }
}

- (void)synchronization {
    @synchronized (self) {
        [self mayThrow];
    }

    @autoreleasepool {
        [self mayThrow];

        @autoreleasepool {
            @synchronized (self) {
                @try {
                    [self mayThrow];
                } @catch (id e) {
                    (void)e;
                }
            }
        }
    }
}
@end

int main(int argc, const char *argv[]) {
    (void)argc; (void)argv;
    @autoreleasepool {
        return 0;
    }
}
