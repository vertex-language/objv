// A tiny configuration parser: the program that exercises error handling
// twice over, once with NSError out-parameters and once with exceptions.
//
// The NSError** pattern is everywhere in Cocoa and is a pointer to a pointer
// the callee writes through, so it is also the everyday test of whether an
// out-parameter survives being written from two frames down.

#import <Foundation/Foundation.h>

NSString *const ConfigErrorDomain = @"ConfigError";

@interface MalformedLine : NSException
@end
@implementation MalformedLine
@end

@interface Config : NSObject
+ (nullable NSDictionary<NSString *, NSString *> *)parse:(NSString *)text
                                                   error:(NSError **)error;
@end

@implementation Config

+ (NSDictionary<NSString *, NSString *> *)parse:(NSString *)text error:(NSError **)error {
	NSMutableDictionary *out = [NSMutableDictionary dictionary];
	NSInteger lineNo = 0;
	for (NSString *raw in [text componentsSeparatedByString:@"\n"]) {
		lineNo++;
		NSString *line = [raw stringByTrimmingCharactersInSet:
		                      [NSCharacterSet whitespaceCharacterSet]];
		if ([line length] == 0 || [line hasPrefix:@"#"]) continue;

		NSRange eq = [line rangeOfString:@"="];
		if (eq.location == NSNotFound) {
			if (error) {
				*error = [NSError errorWithDomain:ConfigErrorDomain
				                             code:lineNo
				                         userInfo:@{NSLocalizedDescriptionKey:
				                                      [NSString stringWithFormat:
				                                        @"line %ld has no '='", (long)lineNo]}];
			}
			return nil;
		}
		NSString *key = [[line substringToIndex:eq.location]
		                  stringByTrimmingCharactersInSet:
		                    [NSCharacterSet whitespaceCharacterSet]];
		NSString *value = [[line substringFromIndex:NSMaxRange(eq)]
		                    stringByTrimmingCharactersInSet:
		                      [NSCharacterSet whitespaceCharacterSet]];
		if ([key length] == 0) {
			@throw [MalformedLine exceptionWithName:@"MalformedLine"
			                                 reason:[NSString stringWithFormat:@"empty key on line %ld", (long)lineNo]
			                               userInfo:nil];
		}
		out[key] = value;
	}
	return out;
}

@end

static void show(NSString *label, NSString *text) {
	NSError *err = nil;
	NSDictionary *cfg = nil;
	@try {
		cfg = [Config parse:text error:&err];
	} @catch (MalformedLine *e) {
		printf("%s: threw %s\n", [label UTF8String], [[e reason] UTF8String]);
		return;
	} @finally {
		printf("%s: parsed\n", [label UTF8String]);
	}

	if (cfg == nil) {
		printf("%s: error %ld: %s\n", [label UTF8String], (long)[err code],
		       [[err localizedDescription] UTF8String]);
		return;
	}
	for (NSString *k in [[cfg allKeys] sortedArrayUsingSelector:@selector(compare:)]) {
		printf("%s: %s=%s\n", [label UTF8String], [k UTF8String], [cfg[k] UTF8String]);
	}
}

int main(void) {
	@autoreleasepool {
		show(@"good", @"# a comment\nhost = example.com\n\nport=8080\n");
		show(@"nokey", @"host = a\n= oops\n");
		show(@"noeq",  @"host = a\nbroken line\n");
	}
	return 0;
}
