// Blocks, as far as they go: nested, returned, stored in a property,
// recursive through a __block variable, and capturing an object that has to
// stay alive because the block does.
//
// The Block ABI is the one place where the compiler emits a data structure a
// library reads at run time and nothing in between checks the shape. A block
// that captures nothing is a global constant; one that captures is a stack
// object with a copy helper; one that escapes has been moved to the heap.
// All three are here.
//
// mode: arc

#import <Foundation/Foundation.h>

typedef NSInteger (^IntOp)(NSInteger);
typedef IntOp (^OpMaker)(NSInteger);

@interface Pipeline : NSObject
@property (nonatomic, copy) IntOp step;
- (NSInteger)runOn:(NSInteger)value;
@end

@implementation Pipeline
- (NSInteger)runOn:(NSInteger)value { return self.step ? self.step(value) : value; }
@end

// A block returning a block, each capturing a parameter of the frame that
// made it — so the outer one has to have been copied for the inner to find
// anything.
static OpMaker adderMaker(void) {
	return ^IntOp(NSInteger base) {
		return ^NSInteger(NSInteger x) { return x + base; };
	};
}

static IntOp compose(IntOp f, IntOp g) {
	return ^NSInteger(NSInteger x) { return g(f(x)); };
}

int main(void) {
	@autoreleasepool {
		// Captures nothing: a global block, and its address is a constant.
		IntOp twice = ^NSInteger(NSInteger x) { return x * 2; };
		printf("twice(21)=%ld\n", (long)twice(21));

		// Captures a local by value: the value at the time of the literal.
		NSInteger offset = 10;
		IntOp shift = ^NSInteger(NSInteger x) { return x + offset; };
		offset = 1000;
		printf("shift(5)=%ld offset=%ld\n", (long)shift(5), (long)offset);

		// __block: one variable, two frames, and the write is visible in both.
		__block NSInteger calls = 0;
		IntOp counted = ^NSInteger(NSInteger x) { calls++; return x; };
		counted(1); counted(2); counted(3);
		printf("calls=%ld\n", (long)calls);

		// Recursion needs the block to see itself, which needs __block.
		__block NSInteger (^fact)(NSInteger);
		fact = ^NSInteger(NSInteger n) { return n < 2 ? 1 : n * fact(n - 1); };
		printf("fact(8)=%ld\n", (long)fact(8));

		// A block that outlives the frame that made it.
		OpMaker make = adderMaker();
		IntOp add7 = make(7);
		printf("add7(35)=%ld composed=%ld\n",
		       (long)add7(35), (long)compose(add7, twice)(0));

		// Stored in a copy property, which is where a stack block would have
		// been a dangling pointer.
		Pipeline *p = [[Pipeline alloc] init];
		{
			NSInteger scale = 3;
			p.step = ^NSInteger(NSInteger x) { return x * scale + offset; };
		}
		printf("pipeline=%ld\n", (long)[p runOn:2]);

		// A block capturing an object, kept alive by a collection.
		NSMutableArray *ops = [NSMutableArray array];
		for (NSInteger i = 1; i <= 3; i++) {
			NSString *label = [NSString stringWithFormat:@"op%ld", (long)i];
			[ops addObject:[^NSInteger(NSInteger x) {
				printf("%s(%ld)\n", [label UTF8String], (long)x);
				return x * i;
			} copy]];
		}
		NSInteger acc = 1;
		for (IntOp op in ops) acc = op(acc);
		printf("acc=%ld\n", (long)acc);

		// A block handed to Foundation, which calls it back.
		NSArray *sorted = [@[@5, @3, @9, @1] sortedArrayUsingComparator:
			^NSComparisonResult(NSNumber *a, NSNumber *b) {
				return [a compare:b];
			}];
		printf("sorted=%s\n",
		       [[sorted componentsJoinedByString:@","] UTF8String]);
	}
	return 0;
}
