// libdispatch, which is the SDK's block-heaviest header and the one that
// takes a block as an argument rather than storing one.
//
// Everything here is deterministic on purpose: a queue used synchronously, a
// group waited on, a semaphore counted down, dispatch_once. No concurrency
// in the output, only in the machinery — which is the part a compiler can be
// wrong about, because a block handed to a C function has to be the layout
// libdispatch reads and not merely one objv agrees with itself about.

#import <Foundation/Foundation.h>
#import <dispatch/dispatch.h>

// dispatch_once is the one initializer pattern every framework uses, and its
// predicate is a long the runtime writes through a pointer.
static NSMutableArray *sharedLog(void) {
	static NSMutableArray *log;
	static dispatch_once_t once;
	dispatch_once(&once, ^{
		log = [[NSMutableArray alloc] init];
		[log addObject:@"created"];
	});
	return log;
}

// A block stored in a variable of dispatch's own typedef, then handed back
// to it.
static dispatch_block_t makeAppender(NSString *what) {
	return [^{ [sharedLog() addObject:what]; } copy];
}

int main(void) {
	@autoreleasepool {
		dispatch_queue_t q = dispatch_queue_create("objv.serial", DISPATCH_QUEUE_SERIAL);

		// Synchronous, so the order is the program's rather than the
		// scheduler's. DISPATCH_NOESCAPE is on the parameter, which is an
		// attribute the header puts there and a compiler has to read past.
		__block NSInteger total = 0;
		for (NSInteger i = 1; i <= 5; i++) {
			dispatch_sync(q, ^{ total += i * i; });
		}
		printf("total=%ld\n", (long)total);

		// dispatch_once, twice.
		printf("log=%s count=%lu\n",
		       [[sharedLog() componentsJoinedByString:@","] UTF8String],
		       (unsigned long)[sharedLog() count]);

		makeAppender(@"alpha")();
		makeAppender(@"beta")();
		printf("log=%s\n", [[sharedLog() componentsJoinedByString:@","] UTF8String]);

		// A group, entered and left by hand and waited on forever — which
		// here means "until the work this thread already did is done".
		dispatch_group_t g = dispatch_group_create();
		__block NSInteger done = 0;
		for (NSInteger i = 0; i < 3; i++) {
			dispatch_group_enter(g);
			dispatch_sync(q, ^{
				done += 1;
				dispatch_group_leave(g);
			});
		}
		dispatch_group_wait(g, DISPATCH_TIME_FOREVER);
		printf("group done=%ld\n", (long)done);

		// A semaphore, which is a counter with a block-free interface.
		dispatch_semaphore_t sem = dispatch_semaphore_create(2);
		printf("sem: %ld %ld %ld\n",
		       (long)dispatch_semaphore_wait(sem, DISPATCH_TIME_NOW),
		       (long)dispatch_semaphore_wait(sem, DISPATCH_TIME_NOW),
		       (long)dispatch_semaphore_wait(sem, DISPATCH_TIME_NOW));
		dispatch_semaphore_signal(sem);
		printf("after signal: %ld\n",
		       (long)dispatch_semaphore_wait(sem, DISPATCH_TIME_NOW));
		// Back to the count it was created with: libdispatch traps on a
		// semaphore released while anything is still held.
		dispatch_semaphore_signal(sem);
		dispatch_semaphore_signal(sem);

		// dispatch_apply, whose iterations may run in any order — so the
		// output is the sum, which does not depend on one.
		// An array cannot be __block — a block captures the pointer, which
		// is what a C array decays to anyway.
		NSInteger squares[8];
		NSInteger *cells = squares;
		dispatch_apply(8, DISPATCH_APPLY_AUTO, ^(size_t i) {
			cells[i] = (NSInteger)(i * i);
		});
		NSInteger sum = 0;
		for (int i = 0; i < 8; i++) sum += squares[i];
		printf("apply sum=%ld\n", (long)sum);

		// Queue-specific data, which is a void* and a destructor function
		// pointer — the C shape beside all the blocks.
		static const char kKey = 0;
		dispatch_queue_set_specific(q, &kKey, (void *)0x2a, NULL);
		printf("specific=%ld\n", (long)(intptr_t)dispatch_queue_get_specific(q, &kKey));

		// A time computed from a constant, which is arithmetic on an opaque
		// integer typedef.
		dispatch_time_t t = dispatch_time(DISPATCH_TIME_NOW, 0);
		printf("time now=%d forever=%d\n",
		       t != DISPATCH_TIME_FOREVER, DISPATCH_TIME_FOREVER != 0);

		printf("label=%s\n", dispatch_queue_get_label(q));
	}
	return 0;
}
