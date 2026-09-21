// The NSError ** out-parameter pattern, with the error made inside the
// callee and read by the caller.
// mode: both
#import <Foundation/Foundation.h>
#include <stdio.h>

static BOOL parse(NSString *s, int *out, NSError **error) {
    NSScanner *sc = [NSScanner scannerWithString:s];
    int v;
    if ([sc scanInt:&v] && sc.isAtEnd) { *out = v; return YES; }
    if (error)
        *error = [NSError errorWithDomain:@"parse" code:s.length
                                 userInfo:@{ NSLocalizedDescriptionKey: [@"bad: " stringByAppendingString:s] }];
    return NO;
}

int main(void) {
    @autoreleasepool {
        NSArray *inputs = @[ @"42", @"4x2", @"-7" ];
        for (NSString *s in inputs) {
            int v = 0;
            NSError *err = nil;
            if (parse(s, &v, &err)) printf("ok %d\n", v);
            else printf("error %s %ld %s\n", err.domain.UTF8String, (long)err.code,
                        err.localizedDescription.UTF8String);
        }
        int v;
        printf("%d\n", parse(@"nope", &v, NULL));
    }
    return 0;
}
