// A cache with a weak owner back-pointer: the memory-management program.
//
// It is the one shape ARC exists for — a parent holding its children
// strongly and each child pointing back weakly — plus the manual half in the
// same file, since a program that manages its own retains has to produce the
// same answers. -dealloc order is part of the output, which is the only way
// to see that the releases happened when they should have.
//
// mode: mrr

#import <Foundation/Foundation.h>

@class Cache;

@interface Entry : NSObject
@property (nonatomic, copy) NSString *key;
@property (nonatomic, assign) NSInteger cost;
@property (nonatomic, assign) __unsafe_unretained Cache *owner;
- (instancetype)initWithKey:(NSString *)key cost:(NSInteger)cost;
@end

@interface Cache : NSObject
@property (nonatomic, readonly) NSInteger totalCost;
- (void)insert:(Entry *)e;
- (Entry *)entryForKey:(NSString *)key;
- (void)evictOver:(NSInteger)budget;
@end

@implementation Entry

- (instancetype)initWithKey:(NSString *)key cost:(NSInteger)cost {
	self = [super init];
	if (self) {
		_key = [key copy];
		_cost = cost;
	}
	return self;
}

- (void)dealloc {
	printf("dealloc %s\n", [_key UTF8String]);
	[_key release];
	[super dealloc];
}

@end

@implementation Cache {
	NSMutableArray<Entry *> *_entries;
}

- (instancetype)init {
	self = [super init];
	if (self) _entries = [[NSMutableArray alloc] init];
	return self;
}

- (void)dealloc {
	[_entries release];
	[super dealloc];
}

- (NSInteger)totalCost {
	NSInteger n = 0;
	for (Entry *e in _entries) n += e.cost;
	return n;
}

- (void)insert:(Entry *)e {
	e.owner = self;
	[_entries addObject:e];
}

- (Entry *)entryForKey:(NSString *)key {
	for (Entry *e in _entries) {
		if ([e.key isEqualToString:key]) return e;
	}
	return nil;
}

- (void)evictOver:(NSInteger)budget {
	while ([self totalCost] > budget && [_entries count] > 0) {
		Entry *victim = _entries[0];
		printf("evict %s (cost %ld)\n", [victim.key UTF8String], (long)victim.cost);
		[_entries removeObjectAtIndex:0];
	}
}

@end

int main(void) {
	NSAutoreleasePool *pool = [[NSAutoreleasePool alloc] init];

	Cache *cache = [[Cache alloc] init];
	NSArray *spec = @[@[@"a", @10], @[@"b", @25], @[@"c", @5], @[@"d", @40]];
	for (NSArray *pair in spec) {
		Entry *e = [[Entry alloc] initWithKey:pair[0] cost:[pair[1] integerValue]];
		[cache insert:e];
		[e release];
	}
	printf("total=%ld\n", (long)[cache totalCost]);

	Entry *found = [cache entryForKey:@"c"];
	printf("found %s owner=%d\n", [found.key UTF8String], found.owner == cache);
	printf("missing=%d\n", [cache entryForKey:@"zz"] == nil);

	[cache evictOver:50];
	printf("after eviction total=%ld\n", (long)[cache totalCost]);

	[cache release];
	printf("cache gone\n");

	[pool release];
	return 0;
}
