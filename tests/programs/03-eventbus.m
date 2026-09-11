// An event bus: blocks stored in a collection and called back later.
//
// The part of Objective-C that a compiler is most likely to get wrong,
// because a block that outlives the statement that wrote it has to have been
// copied to the heap, and one that captures a __block variable has to reach
// the same storage the enclosing scope does.

#import <Foundation/Foundation.h>

typedef void (^Handler)(NSString *payload);

@interface Bus : NSObject
- (void)on:(NSString *)event do:(Handler)handler;
- (void)emit:(NSString *)event payload:(NSString *)payload;
@end

@implementation Bus {
	NSMutableDictionary<NSString *, NSMutableArray *> *_handlers;
}

- (instancetype)init {
	self = [super init];
	if (self) _handlers = [NSMutableDictionary dictionary];
	return self;
}

- (void)on:(NSString *)event do:(Handler)handler {
	NSMutableArray *list = _handlers[event];
	if (list == nil) {
		list = [NSMutableArray array];
		_handlers[event] = list;
	}
	// -copy is what moves a stack block to the heap. A collection holding
	// one that was not copied holds a pointer into a frame that has
	// returned.
	[list addObject:[handler copy]];
}

- (void)emit:(NSString *)event payload:(NSString *)payload {
	for (Handler h in _handlers[event]) h(payload);
}

@end

// A handler built in one frame and called from another, capturing both a
// plain local and a __block one.
static Handler makeCounter(NSString *tag, int *seen) {
	return [^(NSString *payload) {
		*seen += 1;
		printf("[%s] %s (%d)\n", [tag UTF8String], [payload UTF8String], *seen);
	} copy];
}

int main(void) {
	@autoreleasepool {
		Bus *bus = [[Bus alloc] init];

		__block int total = 0;
		[bus on:@"tick" do:^(NSString *p) {
			total += 1;
			printf("tick %s total=%d\n", [p UTF8String], total);
		}];
		[bus on:@"tick" do:^(NSString *p) {
			printf("also tick %s\n", [p UTF8String]);
		}];

		int seen = 0;
		[bus on:@"tock" do:makeCounter(@"tock", &seen)];

		[bus emit:@"tick" payload:@"a"];
		[bus emit:@"tick" payload:@"b"];
		[bus emit:@"tock" payload:@"c"];
		[bus emit:@"missing" payload:@"d"];

		printf("total=%d seen=%d\n", total, seen);
	}
	return 0;
}
