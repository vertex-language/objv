// A plugin registry: protocols, dynamic dispatch through `id`, and the
// introspection a program does when it cannot know the class.
//
// -respondsToSelector: and -conformsToProtocol: are how half of AppKit is
// written, and @selector, NSStringFromSelector and NSStringFromClass are the
// three places a compiler has to agree with the runtime about names.

#import <Foundation/Foundation.h>

@protocol Renderer <NSObject>
@required
- (NSString *)render:(NSString *)input;
@optional
- (NSString *)name;
- (void)reset;
@end

@interface Upper : NSObject <Renderer>
@end
@implementation Upper
- (NSString *)render:(NSString *)input { return [input uppercaseString]; }
- (NSString *)name { return @"upper"; }
@end

@interface Reverser : NSObject <Renderer> {
	NSUInteger _calls;
}
@end
@implementation Reverser
- (NSString *)render:(NSString *)input {
	_calls++;
	NSMutableString *out = [NSMutableString string];
	for (NSInteger i = (NSInteger)[input length] - 1; i >= 0; i--) {
		[out appendFormat:@"%C", [input characterAtIndex:(NSUInteger)i]];
	}
	return out;
}
- (void)reset { _calls = 0; }
- (NSString *)description {
	return [NSString stringWithFormat:@"Reverser(%lu)", (unsigned long)_calls];
}
@end

// Not a Renderer, on purpose: the registry has to notice.
@interface Bystander : NSObject
- (NSString *)render:(NSString *)input;
@end
@implementation Bystander
- (NSString *)render:(NSString *)input { return input; }
@end

static void describe(id obj) {
	printf("%-10s conforms=%d responds(name)=%d responds(reset)=%d\n",
	       [NSStringFromClass([obj class]) UTF8String],
	       [obj conformsToProtocol:@protocol(Renderer)] ? 1 : 0,
	       [obj respondsToSelector:@selector(name)] ? 1 : 0,
	       [obj respondsToSelector:@selector(reset)] ? 1 : 0);
}

int main(void) {
	@autoreleasepool {
		NSArray *candidates = @[[[Upper alloc] init],
		                        [[Reverser alloc] init],
		                        [[Bystander alloc] init]];
		for (id c in candidates) describe(c);

		NSMutableArray<id<Renderer>> *plugins = [NSMutableArray array];
		for (id c in candidates) {
			if ([c conformsToProtocol:@protocol(Renderer)]) [plugins addObject:c];
		}
		printf("registered %lu\n", (unsigned long)[plugins count]);

		for (id<Renderer> p in plugins) {
			NSString *label = [p respondsToSelector:@selector(name)]
			                    ? [p name] : NSStringFromClass([p class]);
			printf("%s -> %s\n", [label UTF8String],
			       [[p render:@"Hello"] UTF8String]);
		}

		// The optional method, twice, and the description that counts.
		id<Renderer> rev = plugins[1];
		[rev render:@"abc"];
		printf("%s\n", [[rev description] UTF8String]);
		if ([rev respondsToSelector:@selector(reset)]) [rev reset];
		printf("%s\n", [[rev description] UTF8String]);

		printf("sel=%s\n", [NSStringFromSelector(@selector(render:)) UTF8String]);
	}
	return 0;
}
