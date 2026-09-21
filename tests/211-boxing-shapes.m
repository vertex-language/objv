// Boxed expressions of every source shape: a char buffer, a char pointer
// variable, an enum, a BOOL, and objc_boxable structs with nested and
// floating members, read back and with their encodings printed.
// mode: both
#import <Foundation/Foundation.h>
#include <stdio.h>
#include <string.h>

typedef NS_ENUM(NSInteger, Mode) { ModeA = 3, ModeB = -7 };
typedef struct __attribute__((objc_boxable)) { float x, y; } P2;
typedef struct __attribute__((objc_boxable)) { P2 origin; double scale; char tag; } Frame;

int main(void) {
    @autoreleasepool {
        char buf[16];
        strcpy(buf, "buffer");
        const char *ptr = "pointer";
        NSString *a = @(buf), *b = @(ptr);
        printf("%s %s %lu\n", a.UTF8String, b.UTF8String, (unsigned long)(a.length + b.length));
        Mode m = ModeB;
        BOOL yes = 3 > 2;
        printf("%ld %d %s\n", (long)@(m).integerValue, @(yes).boolValue, @(yes).description.UTF8String);
        Frame f = { { 1.5f, -2.0f }, 0.25, 'k' };
        NSValue *v = @(f);
        Frame back;
        memset(&back, 0, sizeof back);
        [v getValue:&back size:sizeof back];
        printf("%g %g %g %c\n", back.origin.x, back.origin.y, back.scale, back.tag);
        printf("%s | %s\n", v.objCType, @((P2){ 3, 4 }).objCType);
    }
    return 0;
}
