// A small model layer: value objects in a collection, sorted and printed.
//
// The shape every Cocoa app has — a class with properties, a designated
// initializer, -isEqual:/-hash so the objects can go in a set, -description
// so they print, and a category that adds a computed property the base class
// had no business knowing about.

#import <Foundation/Foundation.h>

@interface Item : NSObject
@property (nonatomic, copy, readonly) NSString *name;
@property (nonatomic, assign, readonly) NSInteger quantity;
@property (nonatomic, assign, readonly) double unitPrice;
- (instancetype)initWithName:(NSString *)name
                    quantity:(NSInteger)quantity
                   unitPrice:(double)unitPrice NS_DESIGNATED_INITIALIZER;
- (instancetype)init NS_UNAVAILABLE;
@end

@implementation Item

- (instancetype)initWithName:(NSString *)name
                    quantity:(NSInteger)quantity
                   unitPrice:(double)unitPrice {
	self = [super init];
	if (self) {
		_name = [name copy];
		_quantity = quantity;
		_unitPrice = unitPrice;
	}
	return self;
}

- (instancetype)init { return [self initWithName:@"" quantity:0 unitPrice:0]; }

- (BOOL)isEqual:(id)other {
	if (self == other) return YES;
	if (![other isKindOfClass:[Item class]]) return NO;
	Item *o = other;
	return [_name isEqualToString:o.name] && _quantity == o.quantity;
}

- (NSUInteger)hash { return [_name hash] ^ (NSUInteger)_quantity; }

- (NSString *)description {
	return [NSString stringWithFormat:@"<Item %@ x%ld @ %.2f>", _name, (long)_quantity, _unitPrice];
}

@end

@interface Item (Money)
@property (nonatomic, readonly) double total;
@end

@implementation Item (Money)
- (double)total { return self.quantity * self.unitPrice; }
@end

int main(void) {
	@autoreleasepool {
		NSArray<Item *> *items = @[
			[[Item alloc] initWithName:@"bolt"   quantity:120 unitPrice:0.15],
			[[Item alloc] initWithName:@"nut"    quantity:300 unitPrice:0.05],
			[[Item alloc] initWithName:@"washer" quantity:80  unitPrice:0.02],
			[[Item alloc] initWithName:@"hinge"  quantity:12  unitPrice:3.40],
		];

		double grand = 0;
		for (Item *it in items) grand += it.total;
		printf("%lu items, total %.2f\n", (unsigned long)items.count, grand);

		NSArray *byValue = [items sortedArrayUsingComparator:^NSComparisonResult(Item *a, Item *b) {
			if (a.total > b.total) return NSOrderedAscending;
			if (a.total < b.total) return NSOrderedDescending;
			return [a.name compare:b.name];
		}];
		for (Item *it in byValue) {
			printf("%-8s %6.2f  %s\n", [it.name UTF8String], it.total,
			       [[it description] UTF8String]);
		}

		// -isEqual: and -hash, through a set that has to use both.
		NSMutableSet *seen = [NSMutableSet set];
		[seen addObject:items[0]];
		[seen addObject:[[Item alloc] initWithName:@"bolt" quantity:120 unitPrice:9.99]];
		printf("distinct: %lu\n", (unsigned long)[seen count]);
		printf("contains hinge: %d\n",
		       [seen containsObject:items[3]] ? 1 : 0);
	}
	return 0;
}
