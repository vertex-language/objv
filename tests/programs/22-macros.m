// The preprocessor, which is half the language by volume in any real
// Objective-C project.
//
// Stringification, token pasting, variadic macros with the comma-swallow
// extension, recursive expansion and the blue-paint rule that stops it,
// __COUNTER__, _Pragma, #pragma pack, __has_feature and __has_include —
// every one of which appears in an SDK header, so a compiler that gets one
// wrong reads a different version of Foundation than clang does.

#import <Foundation/Foundation.h>

#define STR(x)      #x
#define XSTR(x)     STR(x)
#define CAT(a, b)   a##b
#define XCAT(a, b)  CAT(a, b)

#define VERSION_MAJOR 3
#define VERSION_MINOR 14
#define VERSION XSTR(VERSION_MAJOR) "." XSTR(VERSION_MINOR)

// A variadic macro, with the GNU comma-swallow that every logging macro uses.
#define LOG(fmt, ...) printf("[%s:%d] " fmt "\n", __func__, __LINE__, ##__VA_ARGS__)

// Recursion the standard stops: NAME expands to itself once and no further.
#define NAME NAME_IS_NAME
#define NAME_IS_NAME "stopped"

// A macro that builds an identifier, which is how a header declares a family.
#define DECLARE_GETTER(type, name) \
	static type XCAT(get_, name)(void) { return name##_value; }

static int width_value = 42;
static double ratio_value = 1.5;
DECLARE_GETTER(int, width)
DECLARE_GETTER(double, ratio)

// Counters, which is how a header makes a name nobody can collide with.
#define UNIQUE(prefix) XCAT(prefix, __COUNTER__)
static int UNIQUE(tmp_) = 1;
static int UNIQUE(tmp_) = 2;
static int UNIQUE(tmp_) = 3;

// A macro that takes a block, which needs the comma inside braces to survive.
#define REPEAT(n, body) do { for (int _i = 0; _i < (n); _i++) { body } } while (0)

#pragma pack(push, 1)
typedef struct { char a; int b; } Tight;
#pragma pack(pop)
typedef struct { char a; int b; } Loose;

@interface Feature : NSObject
+ (NSString *)report;
@end

@implementation Feature
+ (NSString *)report {
	return [NSString stringWithFormat:@"arc=%d blocks=%d objc2=%d",
#if __has_feature(objc_arc)
	                  1,
#else
	                  0,
#endif
	                  __BLOCKS__,
	                  __OBJC2__];
}
@end

int main(void) {
	@autoreleasepool {
		printf("version=%s\n", VERSION);
		printf("str=%s xstr=%s\n", STR(VERSION_MAJOR), XSTR(VERSION_MAJOR));
		printf("cat=%d\n", CAT(VERSION_, MAJOR));
		printf("name=%s\n", NAME);

		printf("getters: %d %.1f\n", get_width(), get_ratio());
		printf("uniques: %d %d %d\n", tmp_0, tmp_1, tmp_2);

		LOG("no args");
		LOG("one=%d", 7);
		LOG("two=%d,%s", 8, "nine");

		int total = 0;
		REPEAT(4, total += _i; if (_i == 2) continue; total += 10;);
		printf("repeat total=%d\n", total);

		printf("pack: tight=%zu loose=%zu\n", sizeof(Tight), sizeof(Loose));
		printf("%s\n", [[Feature report] UTF8String]);

#if __has_include(<Foundation/NSArray.h>)
		printf("has NSArray.h\n");
#else
		printf("no NSArray.h\n");
#endif

#if defined(__clang__) || defined(__OBJV__)
		printf("known compiler\n");
#endif

		// __FILE__ is a path and would differ between the two builds; the
		// rest of the standard set is a fact about the source.
		printf("line=%d func=%s stdc=%d objc=%d\n",
		       __LINE__, __func__, __STDC__, __OBJC__);

		// Conditional expansion inside a call, and a defined() that has to
		// be evaluated before the macro is expanded.
#define LIMIT (VERSION_MAJOR * 100 + VERSION_MINOR)
		printf("limit=%d\n", LIMIT);
#undef LIMIT
#ifdef LIMIT
		printf("still defined\n");
#else
		printf("undefined\n");
#endif
	}
	return 0;
}
