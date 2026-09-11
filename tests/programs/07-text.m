// Text processing: strings, data, and the formatting that every program
// does and no unit test covers.
//
// -stringWithFormat: is a variadic send, which on this platform passes every
// argument past the declared ones on the stack — so a format with a mix of
// objects, integers and doubles in it is the sharpest ABI test in the corpus,
// and it is also the line every Objective-C program writes ten times.

#import <Foundation/Foundation.h>

@interface Report : NSObject
@property (nonatomic, copy) NSString *title;
@property (nonatomic, strong) NSMutableArray<NSString *> *lines;
- (void)addRow:(NSString *)label value:(double)value count:(NSInteger)count;
- (NSString *)text;
@end

@implementation Report

- (instancetype)init {
	self = [super init];
	if (self) {
		_title = @"untitled";
		_lines = [NSMutableArray array];
	}
	return self;
}

- (void)addRow:(NSString *)label value:(double)value count:(NSInteger)count {
	[_lines addObject:[NSString stringWithFormat:@"%@: %.3f over %ld (%@)",
	                             label, value, (long)count,
	                             count > 1 ? @"many" : @"one"]];
}

- (NSString *)text {
	NSMutableString *out = [NSMutableString stringWithFormat:@"== %@ ==\n", _title];
	for (NSString *line in _lines) [out appendFormat:@"  %@\n", line];
	[out appendFormat:@"(%lu rows)", (unsigned long)[_lines count]];
	return out;
}

@end

int main(void) {
	@autoreleasepool {
		Report *r = [[Report alloc] init];
		r.title = @"Timings";
		[r addRow:@"parse"   value:0.125   count:1];
		[r addRow:@"analyze" value:3.5     count:12];
		[r addRow:@"lower"   value:41.0625 count:340];
		printf("%s\n", [[r text] UTF8String]);

		// Ranges, substrings and comparison.
		NSString *path = @"/usr/local/share/objv/README.md";
		NSRange dot = [path rangeOfString:@"." options:NSBackwardsSearch];
		printf("ext=%s base=%s\n",
		       [[path substringFromIndex:NSMaxRange(dot)] UTF8String],
		       [[[path lastPathComponent] stringByDeletingPathExtension] UTF8String]);

		NSArray *parts = [path componentsSeparatedByString:@"/"];
		printf("depth=%lu third=%s\n", (unsigned long)[parts count],
		       [parts[3] UTF8String]);

		// Data, and a round trip through it.
		NSData *data = [@"objv" dataUsingEncoding:NSUTF8StringEncoding];
		const unsigned char *bytes = [data bytes];
		printf("bytes=%lu %02x%02x%02x%02x b64=%s\n",
		       (unsigned long)[data length], bytes[0], bytes[1], bytes[2], bytes[3],
		       [[data base64EncodedStringWithOptions:0] UTF8String]);

		// Mutation, and the comparison a sort needs.
		NSMutableString *ms = [NSMutableString stringWithString:@"alpha"];
		[ms appendString:@"-beta"];
		[ms replaceOccurrencesOfString:@"a" withString:@"A"
		                       options:0 range:NSMakeRange(0, [ms length])];
		printf("%s cmp=%ld equal=%d\n", [ms UTF8String],
		       (long)[ms compare:@"Alpha-betA"], [ms isEqualToString:@"Alpha-betA"]);

		// Numbers, which is where a format string meets a boxed value.
		NSArray *nums = @[@1, @2.5, @(-3), @YES];
		NSMutableArray *shown = [NSMutableArray array];
		for (NSNumber *n in nums) {
			[shown addObject:[NSString stringWithFormat:@"%@/%d/%.2f",
			                            n, [n intValue], [n doubleValue]]];
		}
		printf("%s\n", [[shown componentsJoinedByString:@" "] UTF8String]);
	}
	return 0;
}
