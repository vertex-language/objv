// A dependency graph, topologically sorted: recursion, mutation during a
// walk, and a weak back-pointer under ARC.
//
// The ARC half of 08: the same shape with the retains removed, plus __weak,
// which is the one ownership qualifier that needs the runtime's side table
// rather than a retain count.
//
// mode: arc

#import <Foundation/Foundation.h>

@class Graph;

@interface Node : NSObject
@property (nonatomic, copy) NSString *name;
@property (nonatomic, strong) NSMutableArray<Node *> *deps;
@property (nonatomic, weak) Graph *graph;
- (instancetype)initWithName:(NSString *)name;
@end

@interface Graph : NSObject
- (Node *)node:(NSString *)name;
- (void)require:(NSString *)name on:(NSString *)dep;
- (NSArray<NSString *> *)order;
@end

// A counter rather than a line per node: the graph holds them in a
// dictionary, and the order a dictionary releases its values in is its
// hash order, which is not a fact about this program.
static int deallocated;

@implementation Node
- (instancetype)initWithName:(NSString *)name {
	self = [super init];
	if (self) {
		_name = [name copy];
		_deps = [NSMutableArray array];
	}
	return self;
}
- (void)dealloc { deallocated++; }
@end

@implementation Graph {
	NSMutableDictionary<NSString *, Node *> *_nodes;
	NSMutableArray<NSString *> *_names; // insertion order, so the walk is stable
}

- (instancetype)init {
	self = [super init];
	if (self) {
		_nodes = [NSMutableDictionary dictionary];
		_names = [NSMutableArray array];
	}
	return self;
}

- (Node *)node:(NSString *)name {
	Node *n = _nodes[name];
	if (n == nil) {
		n = [[Node alloc] initWithName:name];
		n.graph = self;
		_nodes[name] = n;
		[_names addObject:name];
	}
	return n;
}

- (void)require:(NSString *)name on:(NSString *)dep {
	[[self node:name].deps addObject:[self node:dep]];
}

- (void)visit:(Node *)n seen:(NSMutableSet *)seen into:(NSMutableArray *)out {
	if ([seen containsObject:n.name]) return;
	[seen addObject:n.name];
	for (Node *d in n.deps) [self visit:d seen:seen into:out];
	[out addObject:n.name];
}

- (NSArray<NSString *> *)order {
	NSMutableArray *out = [NSMutableArray array];
	NSMutableSet *seen = [NSMutableSet set];
	for (NSString *name in _names) [self visit:_nodes[name] seen:seen into:out];
	return out;
}

@end

int main(void) {
	@autoreleasepool {
		__weak Graph *weakGraph = nil;
		@autoreleasepool {
			Graph *g = [[Graph alloc] init];
			weakGraph = g;

			[g require:@"app"    on:@"objv"];
			[g require:@"objv"   on:@"ir"];
			[g require:@"objv"   on:@"macho"];
			[g require:@"ir"     on:@"arm64"];
			[g require:@"macho"  on:@"arm64"];
			[g require:@"tests"  on:@"app"];

			printf("%s\n", [[[g order] componentsJoinedByString:@" "] UTF8String]);
			printf("graph alive: %d\n", weakGraph != nil);

			Node *ir = [g node:@"ir"];
			printf("ir deps=%lu back=%d\n",
			       (unsigned long)[ir.deps count], ir.graph == g);
		}
		// Every node went with the graph, and the weak reference went to nil
		// rather than to a freed object.
		printf("graph alive: %d deallocated=%d\n", weakGraph != nil, deallocated);
	}
	return 0;
}
