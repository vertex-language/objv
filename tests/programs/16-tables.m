// Static data: the tables a program builds at compile time rather than at
// run time, and the initialization order around them.
//
// A file-scope array of structs holding string pointers, a table of function
// pointers, a static local inside a method, a const object reference, an
// anonymous union in a struct, and +initialize — which the runtime calls
// once, before the first message to the class, and never again.

#import <Foundation/Foundation.h>

typedef enum { OpAdd, OpMul, OpMax } OpKind;

typedef long (*BinOp)(long, long);

static long doAdd(long a, long b) { return a + b; }
static long doMul(long a, long b) { return a * b; }
static long doMax(long a, long b) { return a > b ? a : b; }

typedef struct {
	const char *name;
	OpKind kind;
	BinOp fn;
	long identity;
} OpEntry;

// The table itself, every field a constant expression.
static const OpEntry kOps[] = {
	{"add", OpAdd, doAdd, 0},
	{"mul", OpMul, doMul, 1},
	{"max", OpMax, doMax, -1000},
};
static const size_t kOpCount = sizeof(kOps) / sizeof(kOps[0]);

// A designated initializer at file scope, with a hole the language fills.
typedef struct {
	int width, height, depth;
	const char *label;
} Box3;
static const Box3 kDefaultBox = {.width = 4, .depth = 9, .label = "default"};

// A struct with an anonymous union in it, which is a layout question.
typedef struct {
	int tag;
	union {
		long i;
		double d;
		const char *s;
	};
} Variant;
static const Variant kVariants[] = {
	{0, {.i = 7}},
	{1, {.d = 2.5}},
	{2, {.s = "text"}},
};

// Objective-C constants at file scope, which are addresses the linker fills.
NSString *const kUnitName = @"objv";
static NSString *const kSeparator = @" | ";

@interface Calculator : NSObject
+ (NSUInteger)initCount;
- (long)apply:(OpKind)kind to:(NSArray<NSNumber *> *)values;
- (NSUInteger)nextTicket;
@end

@implementation Calculator

static NSUInteger gInitCalls = 0;

+ (void)initialize {
	// Once per class, before the first message reaches it.
	if (self == [Calculator class]) gInitCalls++;
}

+ (NSUInteger)initCount { return gInitCalls; }

- (long)apply:(OpKind)kind to:(NSArray<NSNumber *> *)values {
	const OpEntry *e = NULL;
	for (size_t i = 0; i < kOpCount; i++) {
		if (kOps[i].kind == kind) { e = &kOps[i]; break; }
	}
	if (!e) return 0;
	long acc = e->identity;
	for (NSNumber *n in values) acc = e->fn(acc, [n longValue]);
	return acc;
}

// A static local: one object for the life of the program, not one per call.
- (NSUInteger)nextTicket {
	static NSUInteger counter = 100;
	return ++counter;
}

@end

int main(void) {
	@autoreleasepool {
		printf("unit=%s sep=[%s]\n", [kUnitName UTF8String], [kSeparator UTF8String]);

		for (size_t i = 0; i < kOpCount; i++) {
			printf("%s: %ld (identity %ld)\n", kOps[i].name,
			       kOps[i].fn(6, 7), kOps[i].identity);
		}

		Calculator *c = [[Calculator alloc] init];
		NSArray *values = @[@3, @1, @4, @1, @5];
		printf("add=%ld mul=%ld max=%ld\n",
		       [c apply:OpAdd to:values],
		       [c apply:OpMul to:values],
		       [c apply:OpMax to:values]);

		printf("box %dx%dx%d %s\n", kDefaultBox.width, kDefaultBox.height,
		       kDefaultBox.depth, kDefaultBox.label);

		for (size_t i = 0; i < 3; i++) {
			const Variant *v = &kVariants[i];
			switch (v->tag) {
			case 0: printf("i=%ld\n", v->i); break;
			case 1: printf("d=%.2f\n", v->d); break;
			default: printf("s=%s\n", v->s); break;
			}
		}

		printf("tickets %lu %lu %lu\n",
		       (unsigned long)[c nextTicket], (unsigned long)[c nextTicket],
		       (unsigned long)[[[Calculator alloc] init] nextTicket]);
		printf("initialize ran %lu time(s)\n", (unsigned long)[Calculator initCount]);

		printf("sizes: OpEntry=%zu Variant=%zu Box3=%zu\n",
		       sizeof(OpEntry), sizeof(Variant), sizeof(Box3));
	}
	return 0;
}
