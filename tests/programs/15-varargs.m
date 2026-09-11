// Variadic functions and variadic methods: the one place where the calling
// convention differs between platforms in a way a program can feel.
//
// On this target every argument past the declared ones is on the stack, so a
// call described as fixed puts them where the callee does not look. A method
// with `...` on it, a C function with `va_list`, a va_copy, a nil-terminated
// list, and a format forwarded to Foundation are five different ways to get
// that wrong.

#import <Foundation/Foundation.h>
#import <stdarg.h>

@interface Logger : NSObject
@property (nonatomic, copy) NSString *prefix;
- (void)log:(NSString *)format, ... NS_FORMAT_FUNCTION(1,2);
- (NSString *)join:(NSString *)first, ... NS_REQUIRES_NIL_TERMINATION;
- (NSInteger)sum:(NSInteger)count, ...;
@end

@implementation Logger

// The forwarding shape: take the list here, hand it to Foundation.
- (void)log:(NSString *)format, ... {
	va_list args;
	va_start(args, format);
	NSString *body = [[NSString alloc] initWithFormat:format arguments:args];
	va_end(args);
	printf("%s%s\n", [self.prefix UTF8String], [body UTF8String]);
}

- (NSString *)join:(NSString *)first, ... {
	NSMutableArray *parts = [NSMutableArray array];
	va_list args;
	va_start(args, first);
	for (NSString *s = first; s != nil; s = va_arg(args, NSString *)) {
		[parts addObject:s];
	}
	va_end(args);
	return [parts componentsJoinedByString:@"/"];
}

// Mixed widths in the tail, which is where a wrong slot shows up as a wrong
// number rather than as a crash.
- (NSInteger)sum:(NSInteger)count, ... {
	NSInteger total = 0;
	va_list args;
	va_start(args, count);
	for (NSInteger i = 0; i < count; i++) {
		total += (NSInteger)va_arg(args, int) + (NSInteger)va_arg(args, double);
	}
	va_end(args);
	return total;
}

@end

// Two walks of one list, which is what va_copy exists for.
static double meanAndMax(int n, double *maxOut, ...) {
	va_list a, b;
	va_start(a, maxOut);
	va_copy(b, a);

	double sum = 0;
	for (int i = 0; i < n; i++) sum += va_arg(a, double);
	va_end(a);

	double max = -1e308;
	for (int i = 0; i < n; i++) {
		double v = va_arg(b, double);
		if (v > max) max = v;
	}
	va_end(b);

	*maxOut = max;
	return n ? sum / n : 0;
}

// Eight integers exhaust the argument registers, so the ninth and the tail
// after it are the stack cases.
static long ninthPlusTail(long a, long b, long c, long d, long e, long f,
                          long g, long h, long i, ...) {
	va_list args;
	va_start(args, i);
	long tail = va_arg(args, long) + va_arg(args, long);
	va_end(args);
	return a + b + c + d + e + f + g + h + i + tail;
}

int main(void) {
	@autoreleasepool {
		Logger *log = [[Logger alloc] init];
		log.prefix = @"[objv] ";
		[log log:@"plain"];
		[log log:@"%@ is %ld and %.2f", @"answer", (long)42, 3.14159];
		[log log:@"%@ %@ %@ %@ %@ %@ %@ %@ %@",
		          @1, @2, @3, @4, @5, @6, @7, @8, @9];

		printf("%s\n", [[log join:@"usr", @"local", @"bin", nil] UTF8String]);
		printf("sum=%ld\n", (long)[log sum:3, 1, 1.5, 2, 2.5, 3, 3.5]);

		double max = 0;
		double mean = meanAndMax(4, &max, 1.0, 9.0, 4.0, 2.0);
		printf("mean=%.2f max=%.2f\n", mean, max);

		printf("ninth=%ld\n", ninthPlusTail(1,2,3,4,5,6,7,8,9, 10L, 20L));

		// Foundation's own nil-terminated list.
		NSArray *a = [NSArray arrayWithObjects:@"x", @"y", @"z", nil];
		printf("array=%s\n", [[a componentsJoinedByString:@","] UTF8String]);

		NSDictionary *d = [NSDictionary dictionaryWithObjectsAndKeys:
		                     @"one", @"1", @"two", @"2", nil];
		printf("dict=%s %s\n", [d[@"1"] UTF8String], [d[@"2"] UTF8String]);
	}
	return 0;
}
