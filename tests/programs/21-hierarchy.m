// A four-level class hierarchy: initializer chains, super in both kinds of
// method, and the dispatch that decides which override runs.
//
// Every one of these is a different edge in the metadata the runtime walks.
// A class method's super goes through the *metaclass* chain and an instance
// method's through the class chain, which are two different links from the
// same object; instancetype makes a factory return the subclass; and
// -dealloc runs from the subclass up, which is the only order that lets a
// subclass read what it is about to release.
//
// mode: mrr

#import <Foundation/Foundation.h>

@interface Shape : NSObject
@property (nonatomic, assign, readonly) NSInteger sides;
+ (NSString *)family;
+ (instancetype)make;
- (instancetype)initWithSides:(NSInteger)sides NS_DESIGNATED_INITIALIZER;
- (double)area;
- (NSString *)render;
@end

@implementation Shape

+ (NSString *)family { return @"shape"; }

// instancetype, and +self: a factory on the base class makes the subclass.
+ (instancetype)make { return [[[self alloc] initWithSides:0] autorelease]; }

- (instancetype)initWithSides:(NSInteger)sides {
	self = [super init];
	if (self) _sides = sides;
	return self;
}

- (instancetype)init { return [self initWithSides:0]; }

- (double)area { return 0; }

// The template method: it calls two the subclasses override, so the answer
// says which method list the runtime found.
- (NSString *)render {
	return [NSString stringWithFormat:@"%@/%@ sides=%ld area=%.2f",
	                  [[self class] family], NSStringFromClass([self class]),
	                  (long)self.sides, [self area]];
}

- (void)dealloc {
	printf("  -dealloc Shape\n");
	[super dealloc];
}

@end

@interface Polygon : Shape
@property (nonatomic, assign) double edge;
- (instancetype)initWithSides:(NSInteger)sides edge:(double)edge NS_DESIGNATED_INITIALIZER;
@end

@implementation Polygon

+ (NSString *)family { return [NSString stringWithFormat:@"%@.polygon", [super family]]; }

- (instancetype)initWithSides:(NSInteger)sides edge:(double)edge {
	// The designated initializer chains to the superclass's designated one.
	self = [super initWithSides:sides];
	if (self) _edge = edge;
	return self;
}

// The inherited designated initializer becomes a secondary one here.
- (instancetype)initWithSides:(NSInteger)sides {
	return [self initWithSides:sides edge:1.0];
}

- (double)area { return self.sides * self.edge * self.edge / 4.0; }

- (void)dealloc {
	printf("  -dealloc Polygon\n");
	[super dealloc];
}

@end

@interface Square : Polygon
- (instancetype)initWithEdge:(double)edge;
@end

@implementation Square

+ (NSString *)family { return [NSString stringWithFormat:@"%@.square", [super family]]; }

- (instancetype)initWithEdge:(double)edge { return [self initWithSides:4 edge:edge]; }

- (double)area { return self.edge * self.edge; }

// An override that calls up and then adds to the answer.
- (NSString *)render {
	return [NSString stringWithFormat:@"[%@]", [super render]];
}

- (void)dealloc {
	printf("  -dealloc Square\n");
	[super dealloc];
}

@end

@interface Tile : Square
@property (nonatomic, copy) NSString *colour;
@end

@implementation Tile

+ (NSString *)family { return [NSString stringWithFormat:@"%@.tile", [super family]]; }

- (instancetype)initWithSides:(NSInteger)sides edge:(double)edge {
	self = [super initWithSides:sides edge:edge];
	if (self) _colour = [@"plain" copy];
	return self;
}

- (NSString *)render {
	return [NSString stringWithFormat:@"%@<%@>", [super render], self.colour];
}

- (void)dealloc {
	printf("  -dealloc Tile\n");
	[_colour release];
	[super dealloc];
}

@end

int main(void) {
	NSAutoreleasePool *pool = [[NSAutoreleasePool alloc] init];

	// A factory on the base class, sent to four classes.
	for (Class k in @[[Shape class], [Polygon class], [Square class], [Tile class]]) {
		id obj = [k make];
		printf("%s\n", [[obj render] UTF8String]);
	}

	printf("families: %s | %s | %s | %s\n",
	       [[Shape family] UTF8String], [[Polygon family] UTF8String],
	       [[Square family] UTF8String], [[Tile family] UTF8String]);

	Tile *t = [[Tile alloc] initWithEdge:3];
	t.colour = @"red";
	printf("%s\n", [[t render] UTF8String]);

	printf("kinds: shape=%d polygon=%d square=%d member-square=%d\n",
	       [t isKindOfClass:[Shape class]], [t isKindOfClass:[Polygon class]],
	       [t isKindOfClass:[Square class]], [t isMemberOfClass:[Square class]]);
	printf("responds: initWithEdge=%d area=%d missing=%d\n",
	       [t respondsToSelector:@selector(initWithEdge:)],
	       [t respondsToSelector:@selector(area)],
	       [t respondsToSelector:@selector(volume)]);
	printf("super chain: %s <- %s <- %s <- %s\n",
	       [NSStringFromClass([t class]) UTF8String],
	       [NSStringFromClass([[t class] superclass]) UTF8String],
	       [NSStringFromClass([[[t class] superclass] superclass]) UTF8String],
	       [NSStringFromClass([[[[t class] superclass] superclass] superclass]) UTF8String]);

	printf("releasing tile:\n");
	[t release];

	[pool release];
	return 0;
}
