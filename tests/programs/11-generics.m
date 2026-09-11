// Lightweight generics, KVC, and the collection API a real program leans on:
// the parts of Objective-C that are erased at run time and therefore have to
// be exactly right at compile time, because nothing will catch them later.

#import <Foundation/Foundation.h>

@interface Person : NSObject
@property (nonatomic, copy) NSString *name;
@property (nonatomic, assign) NSInteger age;
@property (nonatomic, copy) NSArray<NSString *> *tags;
+ (instancetype)named:(NSString *)name age:(NSInteger)age tags:(NSArray<NSString *> *)tags;
@end

@implementation Person
+ (instancetype)named:(NSString *)name age:(NSInteger)age tags:(NSArray<NSString *> *)tags {
	Person *p = [[self alloc] init];
	p.name = name;
	p.age = age;
	p.tags = tags;
	return p;
}
- (NSString *)description { return [NSString stringWithFormat:@"%@(%ld)", _name, (long)_age]; }
@end

// A generic container of its own, which is where the erasure is visible: the
// type argument is a compile-time promise and the storage is plain `id`.
@interface Box<__covariant T> : NSObject
- (void)put:(T)value;
- (T)peek;
- (NSArray<T> *)all;
@end

@implementation Box {
	NSMutableArray *_items;
}
- (instancetype)init {
	self = [super init];
	if (self) _items = [NSMutableArray array];
	return self;
}
- (void)put:(id)value { [_items addObject:value]; }
- (id)peek { return [_items lastObject]; }
- (NSArray *)all { return [_items copy]; }
@end

int main(void) {
	@autoreleasepool {
		NSArray<Person *> *people = @[
			[Person named:@"ada"    age:36 tags:@[@"math", @"engine"]],
			[Person named:@"grace"  age:45 tags:@[@"navy", @"compiler"]],
			[Person named:@"alan"   age:41 tags:@[@"math"]],
		];

		// Key-value coding, which resolves a property by name at run time.
		printf("names=%s\n",
		       [[[people valueForKey:@"name"] componentsJoinedByString:@","] UTF8String]);
		printf("total age=%ld max=%ld\n",
		       (long)[[people valueForKeyPath:@"@sum.age"] integerValue],
		       (long)[[people valueForKeyPath:@"@max.age"] integerValue]);

		// Filtering and mapping the way a program actually writes it.
		NSMutableArray<NSString *> *mathy = [NSMutableArray array];
		for (Person *p in people) {
			if ([p.tags containsObject:@"math"]) [mathy addObject:p.name];
		}
		printf("math=%s\n", [[mathy componentsJoinedByString:@","] UTF8String]);

		NSArray *byAge = [people sortedArrayUsingDescriptors:
		                    @[[NSSortDescriptor sortDescriptorWithKey:@"age" ascending:NO]]];
		printf("oldest=%s\n", [[byAge[0] description] UTF8String]);

		// Enumeration with an index and an early stop.
		[people enumerateObjectsUsingBlock:^(Person *p, NSUInteger i, BOOL *stop) {
			printf("%lu: %s\n", (unsigned long)i, [p.name UTF8String]);
			if (i == 1) *stop = YES;
		}];

		// The generic container, specialized two ways in one function.
		Box<NSString *> *strings = [[Box alloc] init];
		[strings put:@"one"];
		[strings put:@"two"];
		printf("peek=%s count=%lu\n",
		       [[strings peek] UTF8String], (unsigned long)[[strings all] count]);

		Box<NSNumber *> *numbers = [[Box alloc] init];
		[numbers put:@(7)];
		printf("numeric peek=%d\n", [[numbers peek] intValue]);

		// A dictionary of arrays, grouped and printed in a stable order.
		NSMutableDictionary<NSString *, NSMutableArray<NSString *> *> *byTag =
			[NSMutableDictionary dictionary];
		for (Person *p in people) {
			for (NSString *tag in p.tags) {
				NSMutableArray *list = byTag[tag];
				if (!list) { list = [NSMutableArray array]; byTag[tag] = list; }
				[list addObject:p.name];
			}
		}
		for (NSString *tag in [[byTag allKeys] sortedArrayUsingSelector:@selector(compare:)]) {
			printf("%s: %s\n", [tag UTF8String],
			       [[byTag[tag] componentsJoinedByString:@","] UTF8String]);
		}
	}
	return 0;
}
