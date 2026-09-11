// A state machine driven by a table of blocks: switch, goto, nested loops,
// and the control flow a compiler most often gets subtly wrong.
//
// Every exit from the middle of something is here — break out of a switch
// inside a loop, continue past the rest of an iteration, a goto out of two
// loops, a return from inside a @try — because each of them is a different
// edge and the one that is missing is never the one you expected.

#import <Foundation/Foundation.h>

typedef NS_ENUM(NSInteger, State) {
	StateIdle,
	StateRunning,
	StatePaused,
	StateDone,
};

static const char *nameOf(State s) {
	switch (s) {
	case StateIdle:    return "idle";
	case StateRunning: return "running";
	case StatePaused:  return "paused";
	case StateDone:    return "done";
	}
	return "?";
}

@interface Machine : NSObject
@property (nonatomic, assign) State state;
@property (nonatomic, assign) NSInteger ticks;
- (BOOL)step:(char)input;
@end

@implementation Machine

- (BOOL)step:(char)input {
	switch (self.state) {
	case StateIdle:
		if (input == 's') { self.state = StateRunning; return YES; }
		break;
	case StateRunning:
		switch (input) {
		case 'p': self.state = StatePaused;  return YES;
		case 'x': self.state = StateDone;    return YES;
		case '.': self.ticks++;              return YES;
		default:  break;
		}
		break;
	case StatePaused:
		if (input == 'r') { self.state = StateRunning; return YES; }
		if (input == 'x') { self.state = StateDone;    return YES; }
		break;
	case StateDone:
		break;
	}
	return NO;
}

@end

// Two loops and a goto out of both, which is the one control transfer a
// structured break cannot express.
static NSString *firstPair(NSArray<NSString *> *words, NSUInteger len) {
	NSString *found = nil;
	for (NSString *a in words) {
		for (NSString *b in words) {
			if (a == b) continue;
			if ([a length] + [b length] == len) {
				found = [NSString stringWithFormat:@"%@%@", a, b];
				goto done;
			}
		}
	}
done:
	return found ?: @"(none)";
}

static NSInteger counted(NSArray<NSNumber *> *xs) {
	NSInteger sum = 0;
	for (NSNumber *n in xs) {
		NSInteger v = [n integerValue];
		if (v < 0) continue;          // skip
		if (v > 100) break;           // stop
		for (NSInteger i = 0; i < v; i++) {
			if (i % 3 == 0) continue;
			sum += i;
		}
	}
	return sum;
}

int main(void) {
	@autoreleasepool {
		Machine *m = [[Machine alloc] init];
		const char *script = "zs..p.rx.s";
		for (const char *p = script; *p; p++) {
			BOOL ok = [m step:*p];
			printf("%c -> %-7s %s ticks=%ld\n", *p, nameOf(m.state),
			       ok ? "ok" : "ignored", (long)m.ticks);
		}

		NSArray *words = @[@"al", @"bob", @"carol", @"dee"];
		printf("pair5=%s pair99=%s\n",
		       [firstPair(words, 5) UTF8String],
		       [firstPair(words, 99) UTF8String]);

		printf("counted=%ld\n", (long)counted(@[@4, @(-2), @7, @250, @9]));

		// A do/while with a break, and a ternary chain.
		NSInteger n = 0, guard = 0;
		do {
			n = n * 3 + 1;
			if (++guard > 10) break;
		} while (n < 100);
		printf("n=%ld guard=%ld class=%s\n", (long)n, (long)guard,
		       n < 10 ? "small" : n < 100 ? "medium" : "large");
	}
	return 0;
}
