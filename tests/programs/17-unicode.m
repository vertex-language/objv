// Strings that are not ASCII, and the three things a compiler has to get
// right about them.
//
// A @"…" whose characters do not fit in a byte changes encoding, section and
// length field all at once: the object is a CFString with UTF-16 characters
// beside it and a count of code units rather than bytes. @encode is the
// other compile-time string — a type written out in the runtime's own
// notation, which nothing but a compiler can produce.

#import <Foundation/Foundation.h>

typedef struct { int a; double b; char c[4]; } Mixed;

@interface Label : NSObject
@property (nonatomic, copy) NSString *text;
- (NSUInteger)codeUnits;
- (NSUInteger)composedCharacters;
@end

@implementation Label
- (NSUInteger)codeUnits { return [_text length]; }
- (NSUInteger)composedCharacters {
	__block NSUInteger n = 0;
	[_text enumerateSubstringsInRange:NSMakeRange(0, [_text length])
	                          options:NSStringEnumerationByComposedCharacterSequences
	                       usingBlock:^(NSString *s, NSRange r, NSRange e, BOOL *stop) {
		n++;
	}];
	return n;
}
@end

int main(void) {
	@autoreleasepool {
		// ASCII: one byte per character, and the literal is 8-bit data.
		NSString *plain = @"hello";
		// Not ASCII: UTF-16, and the length counts code units.
		NSString *accents = @"crème brûlée";
		// Outside the BMP: one character, two code units.
		NSString *emoji = @"a\U0001F600b";

		Label *l = [[Label alloc] init];
		for (NSString *s in @[plain, accents, emoji]) {
			l.text = s;
			printf("%-16s units=%lu chars=%lu utf8=%lu\n", [s UTF8String],
			       (unsigned long)[l codeUnits],
			       (unsigned long)[l composedCharacters],
			       (unsigned long)[s lengthOfBytesUsingEncoding:NSUTF8StringEncoding]);
		}

		// Escapes, which the scanner decodes and the literal re-encodes.
		NSString *escaped = @"tab\there\nnewline é \U0001F4A1 \\ \"quoted\"";
		printf("escaped units=%lu\n", (unsigned long)[escaped length]);
		printf("first=%C last=%C\n",
		       [escaped characterAtIndex:0],
		       [escaped characterAtIndex:[escaped length] - 1]);

		// Comparison with options, which is where a program actually does
		// case folding rather than lowercasing by hand.
		printf("caseless=%ld numeric=%ld\n",
		       (long)[@"Straße" compare:@"STRASSE" options:NSCaseInsensitiveSearch],
		       (long)[@"file10" compare:@"file9" options:NSNumericSearch]);

		// The one string a compiler invents: @encode.
		printf("int=%s double=%s ptr=%s\n",
		       @encode(int), @encode(double), @encode(char *));
		printf("array=%s struct=%s\n", @encode(int[4]), @encode(Mixed));
		printf("id=%s sel=%s class=%s\n", @encode(id), @encode(SEL), @encode(Class));
		printf("block=%s\n", @encode(void (^)(int)));

		// And the names the runtime keeps.
		printf("class=%s sel=%s\n",
		       [NSStringFromClass([Label class]) UTF8String],
		       [NSStringFromSelector(@selector(composedCharacters)) UTF8String]);

		// Round trip through UTF-16 data, which is the encoding the literal
		// already used.
		NSData *utf16 = [accents dataUsingEncoding:NSUTF16LittleEndianStringEncoding];
		NSString *back = [[NSString alloc] initWithData:utf16
		                                       encoding:NSUTF16LittleEndianStringEncoding];
		printf("roundtrip=%d bytes=%lu\n", [back isEqualToString:accents],
		       (unsigned long)[utf16 length]);
	}
	return 0;
}
