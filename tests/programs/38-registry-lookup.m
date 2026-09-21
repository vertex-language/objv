// A registry of live objects by number, the shape of a window system's
// bookkeeping: a static dictionary, a lookup function that returns what the
// dictionary holds, and callers that look an object up many times over its
// life. Whatever a lookup hands back is borrowed, so once the object leaves
// the registry nothing may keep it alive: it is freed exactly when removed.
// mode: arc
#import <Foundation/Foundation.h>
#include <stdio.h>

@interface Session : NSObject
@property (nonatomic) int ident;
@property (nonatomic) int hits;
@end

@implementation Session
- (void)dealloc {
    printf("session %d freed after %d hits\n", self.ident, self.hits);
}
@end

static NSMutableDictionary* sessions;
static int nextIdent = 1;

static Session* sessionOf(int ident) {
    return sessions[@(ident)];
}

static Session* firstSession(void) {
    Session* found = nil;
    for (NSNumber* key in [[sessions allKeys] sortedArrayUsingSelector:@selector(compare:)]) {
        found = sessionOf([key intValue]);
        break;
    }
    return found;
}

static int openSession(void) {
    @autoreleasepool {
        if (sessions == nil)
            sessions = [NSMutableDictionary dictionary];
        Session* s = [[Session alloc] init];
        s.ident = nextIdent++;
        sessions[@(s.ident)] = s;
        return s.ident;
    }
}

static void touch(int ident) {
    Session* s = sessionOf(ident);
    if (s != nil)
        s.hits = s.hits + 1;
}

static void closeSession(int ident) {
    @autoreleasepool {
        printf("closing %d\n", ident);
        [sessions removeObjectForKey:@(ident)];
        printf("closed %d\n", ident);
    }
}

int main(void) {
    int a = openSession();
    int b = openSession();
    for (int i = 0; i < 1000; i++) {
        @autoreleasepool {
            touch(a);
            if (i % 3 == 0)
                touch(b);
        }
    }
    @autoreleasepool {
        Session* f = firstSession();
        printf("first is %d with %d hits\n", f.ident, f.hits);
    }
    closeSession(a);
    closeSession(b);
    printf("sessions left: %lu\n", (unsigned long)[sessions count]);
    return 0;
}
