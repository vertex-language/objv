// CoreFoundation, and the bridge between it and Objective-C.
//
// CF is a C API over the same objects Foundation vends, which makes
// __bridge the one cast in the language that is about ownership and nothing
// else. Its headers are a different dialect from Foundation's: opaque struct
// pointers behind typedefs, CF_RETURNS_RETAINED and CF_BRIDGED_TYPE on
// nearly every declaration, function pointers in callback structures, and a
// constant of its own kind for every type.
//
// mode: arc

#import <Foundation/Foundation.h>
#import <CoreFoundation/CoreFoundation.h>

// A callback structure of function pointers, which is how CF takes a
// comparator — the C shape of what a block does on the other side.
static CFComparisonResult compareLengths(const void *a, const void *b, void *ctx) {
	CFIndex la = CFStringGetLength((CFStringRef)a);
	CFIndex lb = CFStringGetLength((CFStringRef)b);
	(void)ctx;
	if (la != lb) return la < lb ? kCFCompareLessThan : kCFCompareGreaterThan;
	return CFStringCompare((CFStringRef)a, (CFStringRef)b, 0);
}

int main(void) {
	@autoreleasepool {
		// A CF string built by CF, read by Foundation. __bridge alone: no
		// ownership changes hands, because the object is already in a pool.
		CFStringRef cf = CFStringCreateWithCString(kCFAllocatorDefault,
		                                           "corefoundation",
		                                           kCFStringEncodingUTF8);
		NSString *ns = (__bridge NSString *)cf;
		printf("cf=%ld ns=%lu same=%d\n",
		       (long)CFStringGetLength(cf), (unsigned long)[ns length],
		       [ns isEqualToString:@"corefoundation"]);

		// And back, with the ownership stated. CFBridgingRetain hands ARC's
		// reference to CF; the CFRelease below is what balances it.
		NSString *made = [NSString stringWithFormat:@"%@-%d", ns, 2];
		CFStringRef owned = (__bridge_retained CFStringRef)made;
		printf("owned=%ld\n", (long)CFStringGetLength(owned));
		CFRelease(owned);

		// A CF array, sorted by a C function pointer, printed through
		// Foundation.
		CFMutableArrayRef words = CFArrayCreateMutable(kCFAllocatorDefault, 0,
		                                               &kCFTypeArrayCallBacks);
		for (NSString *w in @[@"delta", @"a", @"charlie", @"bb"]) {
			CFArrayAppendValue(words, (__bridge const void *)w);
		}
		CFArraySortValues(words, CFRangeMake(0, CFArrayGetCount(words)),
		                  compareLengths, NULL);
		printf("sorted=%s\n",
		       [[(__bridge NSArray *)words componentsJoinedByString:@","] UTF8String]);

		// A CF dictionary read as an NSDictionary, keys sorted so the order
		// is a fact about the data.
		CFMutableDictionaryRef d = CFDictionaryCreateMutable(kCFAllocatorDefault, 0,
			&kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
		CFDictionarySetValue(d, CFSTR("one"), CFSTR("1"));
		CFDictionarySetValue(d, CFSTR("two"), CFSTR("2"));
		NSDictionary *nd = (__bridge NSDictionary *)d;
		for (NSString *k in [[nd allKeys] sortedArrayUsingSelector:@selector(compare:)]) {
			printf("%s=%s ", [k UTF8String], [nd[k] UTF8String]);
		}
		printf("\n");

		// The type system CF has instead of classes.
		printf("ids: string=%d array=%d dict=%d\n",
		       CFGetTypeID(cf) == CFStringGetTypeID(),
		       CFGetTypeID(words) == CFArrayGetTypeID(),
		       CFGetTypeID(d) == CFDictionaryGetTypeID());
		// __bridge_transfer is the other direction of the ownership cast:
		// CFCopy* returns +1 and ARC takes it.
		NSString *desc = (__bridge_transfer NSString *)
		                   CFCopyTypeIDDescription(CFStringGetTypeID());
		printf("desc=%s\n", [desc UTF8String]);

		// Numbers and booleans, which CF boxes differently from Foundation.
		int n = 42;
		CFNumberRef num = CFNumberCreate(kCFAllocatorDefault, kCFNumberIntType, &n);
		int back = 0;
		CFNumberGetValue(num, kCFNumberIntType, &back);
		printf("number=%d bridged=%d bool=%d\n", back,
		       [(__bridge NSNumber *)num intValue],
		       CFBooleanGetValue(kCFBooleanTrue));

		// A range, which is a struct CF passes by value everywhere.
		CFRange r = CFRangeMake(3, 5);
		printf("range=%ld,%ld size=%zu\n", (long)r.location, (long)r.length, sizeof(CFRange));

		CFRelease(num);
		CFRelease(d);
		CFRelease(words);
		CFRelease(cf);
	}
	return 0;
}
