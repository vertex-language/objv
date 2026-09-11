// A pretty-printer for a heterogeneous tree: the shape half of Cocoa has,
// where a value is any of six classes and the code asks which.
//
// Recursion through `id`, isKindOfClass: dispatch, NSNull standing in for
// nothing, mutual recursion between two functions, a depth limit, and the
// sorting a deterministic dump needs. It is also the program that nests
// containers deeply enough for a recursive lowering to run out of whatever
// it was going to run out of.

#import <Foundation/Foundation.h>

static void dumpValue(id value, NSUInteger indent, NSMutableString *out);

static void pad(NSUInteger n, NSMutableString *out) {
	for (NSUInteger i = 0; i < n; i++) [out appendString:@"  "];
}

static void dumpDictionary(NSDictionary *d, NSUInteger indent, NSMutableString *out) {
	if ([d count] == 0) { [out appendString:@"{}"]; return; }
	[out appendString:@"{\n"];
	NSArray *keys = [[d allKeys] sortedArrayUsingSelector:@selector(compare:)];
	for (NSUInteger i = 0; i < [keys count]; i++) {
		pad(indent + 1, out);
		[out appendFormat:@"\"%@\": ", keys[i]];
		dumpValue(d[keys[i]], indent + 1, out);
		if (i + 1 < [keys count]) [out appendString:@","];
		[out appendString:@"\n"];
	}
	pad(indent, out);
	[out appendString:@"}"];
}

static void dumpArray(NSArray *a, NSUInteger indent, NSMutableString *out) {
	if ([a count] == 0) { [out appendString:@"[]"]; return; }
	[out appendString:@"[\n"];
	[a enumerateObjectsUsingBlock:^(id v, NSUInteger i, BOOL *stop) {
		pad(indent + 1, out);
		dumpValue(v, indent + 1, out);
		if (i + 1 < [a count]) [out appendString:@","];
		[out appendString:@"\n"];
	}];
	pad(indent, out);
	[out appendString:@"]"];
}

static void dumpValue(id value, NSUInteger indent, NSMutableString *out) {
	if (value == nil || [value isKindOfClass:[NSNull class]]) {
		[out appendString:@"null"];
	} else if ([value isKindOfClass:[NSDictionary class]]) {
		dumpDictionary(value, indent, out);
	} else if ([value isKindOfClass:[NSArray class]]) {
		dumpArray(value, indent, out);
	} else if ([value isKindOfClass:[NSString class]]) {
		[out appendFormat:@"\"%@\"", value];
	} else if ([value isKindOfClass:[NSNumber class]]) {
		NSNumber *n = value;
		if (strcmp([n objCType], @encode(BOOL)) == 0) {
			[out appendString:[n boolValue] ? @"true" : @"false"];
		} else if (strcmp([n objCType], @encode(double)) == 0) {
			[out appendFormat:@"%.3f", [n doubleValue]];
		} else {
			[out appendFormat:@"%ld", (long)[n longValue]];
		}
	} else {
		[out appendFormat:@"<%@>", NSStringFromClass([value class])];
	}
}

// Depth, counted by walking rather than by remembering: mutual recursion
// with the container walk above.
static NSUInteger depthOf(id value) {
	if ([value isKindOfClass:[NSDictionary class]]) {
		NSUInteger best = 0;
		for (id k in value) {
			NSUInteger d = depthOf([value objectForKey:k]);
			if (d > best) best = d;
		}
		return best + 1;
	}
	if ([value isKindOfClass:[NSArray class]]) {
		NSUInteger best = 0;
		for (id v in value) {
			NSUInteger d = depthOf(v);
			if (d > best) best = d;
		}
		return best + 1;
	}
	return 0;
}

// A tree built by recursion, so the nesting is not written out by hand.
static id nest(NSUInteger depth) {
	if (depth == 0) return @"leaf";
	return @{@"depth": @(depth), @"child": nest(depth - 1), @"siblings": @[@1, nest(depth - 1)]};
}

int main(void) {
	@autoreleasepool {
		id doc = @{
			@"name":    @"objv",
			@"version": @3,
			@"ratio":   @2.5,
			@"stable":  @NO,
			@"authors": @[@"ada", @"grace"],
			@"nothing": [NSNull null],
			@"empty":   @[],
			@"nested":  @{@"a": @[@1, @[@2, @[@3]]], @"b": @{}},
		};

		NSMutableString *out = [NSMutableString string];
		dumpValue(doc, 0, out);
		printf("%s\n", [out UTF8String]);
		printf("depth=%lu\n", (unsigned long)depthOf(doc));

		NSMutableString *deep = [NSMutableString string];
		id tree = nest(4);
		dumpValue(tree, 0, deep);
		printf("built depth=%lu length=%lu\n",
		       (unsigned long)depthOf(tree), (unsigned long)[deep length]);

		// An object that is none of the six, so the fallback runs.
		NSMutableString *odd = [NSMutableString string];
		dumpValue([[NSObject alloc] init], 0, odd);
		printf("other=%s\n", [odd UTF8String]);
	}
	return 0;
}
