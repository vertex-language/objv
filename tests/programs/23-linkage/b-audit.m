// The category, in a unit of its own: the runtime attaches its methods to a
// class this file did not compile, and -deposit: over in a-model.m reaches
// -didChange through it.

#import "shared.h"

@implementation Account (Audit)

// One log for every account, keyed by name, so the category can carry state
// without an ivar — which is what a category cannot have.
static NSMutableDictionary<NSString *, NSMutableArray *> *gLogs;

+ (void)load {
	gLogs = [[NSMutableDictionary alloc] init];
}

- (NSArray<NSString *> *)log {
	return gLogs[[self name]] ?: @[];
}

- (void)note:(NSString *)what {
	NSMutableArray *entries = gLogs[[self name]];
	if (entries == nil) {
		entries = [NSMutableArray array];
		gLogs[[self name]] = entries;
	}
	[entries addObject:what];
}

// The override the base class calls. Nothing in a-model.m knows it is here.
- (void)didChange {
	[self note:[NSString stringWithFormat:@"balance=%ld", (long)[self balance]]];
}

@end
