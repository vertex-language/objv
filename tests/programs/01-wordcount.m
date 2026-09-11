// A word counter: the program everyone writes first, and the one that puts
// the most of Foundation's string and collection API in one place.
//
// NSString tokenizing, NSMutableDictionary with NSNumber values, sorting an
// array of keys with a comparator block that reaches two levels of capture,
// and a format string with a width and a precision in it.

#import <Foundation/Foundation.h>

static NSArray<NSString *> *wordsIn(NSString *text) {
	NSMutableArray *words = [NSMutableArray array];
	NSCharacterSet *seps = [NSCharacterSet characterSetWithCharactersInString:@" \t\n,.;:!?\"'()"];
	for (NSString *piece in [text componentsSeparatedByCharactersInSet:seps]) {
		if ([piece length] == 0) continue;
		[words addObject:[piece lowercaseString]];
	}
	return words;
}

static NSDictionary<NSString *, NSNumber *> *tally(NSArray<NSString *> *words) {
	NSMutableDictionary *counts = [NSMutableDictionary dictionary];
	for (NSString *w in words) {
		NSNumber *seen = counts[w];
		counts[w] = @([seen intValue] + 1);
	}
	return counts;
}

int main(void) {
	@autoreleasepool {
		NSString *text = @"the quick brown fox jumps over the lazy dog; "
		                 @"the dog barks, and the fox runs. The end.";
		NSArray *words = wordsIn(text);
		NSDictionary *counts = tally(words);

		// Most frequent first, ties broken alphabetically, so the order is
		// a fact about the data rather than about the hash table.
		NSArray *keys = [[counts allKeys] sortedArrayUsingComparator:^NSComparisonResult(id a, id b) {
			int ca = [counts[a] intValue], cb = [counts[b] intValue];
			if (ca != cb) return ca > cb ? NSOrderedAscending : NSOrderedDescending;
			return [a compare:b];
		}];

		printf("%lu words, %lu distinct\n",
		       (unsigned long)[words count], (unsigned long)[keys count]);
		int shown = 0;
		for (NSString *k in keys) {
			printf("%-8s %d\n", [k UTF8String], [counts[k] intValue]);
			if (++shown == 6) break;
		}
	}
	return 0;
}
