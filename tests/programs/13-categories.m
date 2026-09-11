// Categories and class extensions on classes the program did not write —
// the mechanism half of Cocoa is built out of, and the one that has to agree
// with a runtime that already loaded the class.
//
// A category on NSString, one on NSArray, a class extension declaring
// private state, a class method, and a subclass that overrides a method its
// superclass calls — which is the only way to see that the method list the
// runtime attached is the one that was emitted.

#import <Foundation/Foundation.h>

@interface NSString (Objv)
- (NSString *)objv_repeated:(NSUInteger)times;
- (NSUInteger)objv_vowels;
+ (NSString *)objv_joinNumbers:(NSArray<NSNumber *> *)numbers;
@end

@implementation NSString (Objv)

- (NSString *)objv_repeated:(NSUInteger)times {
	NSMutableString *s = [NSMutableString string];
	for (NSUInteger i = 0; i < times; i++) [s appendString:self];
	return s;
}

- (NSUInteger)objv_vowels {
	NSUInteger n = 0;
	for (NSUInteger i = 0; i < [self length]; i++) {
		unichar c = [self characterAtIndex:i];
		if (c=='a'||c=='e'||c=='i'||c=='o'||c=='u') n++;
	}
	return n;
}

+ (NSString *)objv_joinNumbers:(NSArray<NSNumber *> *)numbers {
	NSMutableArray *parts = [NSMutableArray array];
	for (NSNumber *n in numbers) [parts addObject:[n stringValue]];
	return [parts componentsJoinedByString:@"+"];
}

@end

@interface NSArray<ObjectType> (Objv)
- (NSArray *)objv_reversed;
@end

@implementation NSArray (Objv)
- (NSArray *)objv_reversed {
	NSMutableArray *out = [NSMutableArray arrayWithCapacity:[self count]];
	for (NSUInteger i = [self count]; i > 0; i--) [out addObject:self[i - 1]];
	return out;
}
@end

// A class whose superclass calls a method the subclass overrides: the
// template-method shape, and the one that proves dispatch reached the right
// method list.
@interface Formatter : NSObject
- (NSString *)format:(NSArray<NSNumber *> *)values;   // calls -decorate:
- (NSString *)decorate:(NSString *)s;
@end

@implementation Formatter
- (NSString *)format:(NSArray<NSNumber *> *)values {
	return [self decorate:[NSString objv_joinNumbers:values]];
}
- (NSString *)decorate:(NSString *)s { return s; }
@end

// A class extension: state and a method nobody outside this file can see.
@interface Bracketed : Formatter
@end

@interface Bracketed ()
@property (nonatomic, assign) NSUInteger calls;
- (NSString *)brackets:(NSString *)s;
@end

@implementation Bracketed
- (NSString *)brackets:(NSString *)s { return [NSString stringWithFormat:@"[%@]", s]; }
- (NSString *)decorate:(NSString *)s {
	self.calls++;
	return [self brackets:[super decorate:s]];
}
@end

int main(void) {
	@autoreleasepool {
		printf("%s\n", [[@"ab" objv_repeated:3] UTF8String]);
		printf("vowels=%lu\n", (unsigned long)[@"objective see" objv_vowels]);
		printf("%s\n", [[NSString objv_joinNumbers:@[@1, @2, @3]] UTF8String]);

		NSArray *r = [@[@"a", @"b", @"c"] objv_reversed];
		printf("%s\n", [[r componentsJoinedByString:@""] UTF8String]);

		Formatter *plain = [[Formatter alloc] init];
		Bracketed *fancy = [[Bracketed alloc] init];
		NSArray *values = @[@4, @5, @6];
		printf("%s | %s\n",
		       [[plain format:values] UTF8String],
		       [[fancy format:values] UTF8String]);
		[fancy format:values];
		printf("calls=%lu isKind=%d isMember=%d\n",
		       (unsigned long)fancy.calls,
		       [fancy isKindOfClass:[Formatter class]],
		       [fancy isMemberOfClass:[Formatter class]]);

		// A category method on an instance the program never made.
		printf("literal=%s\n", [[@"x" objv_repeated:4] UTF8String]);
	}
	return 0;
}
