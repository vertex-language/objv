#import <Foundation/Foundation.h>

extern NSString *const AccountDomain;
extern int accountsCreated;
extern double accountRate;

@interface Account : NSObject {
@protected
	NSString *_name;
	NSInteger _balance;
}
+ (instancetype)named:(NSString *)name;
- (NSString *)name;
- (NSInteger)balance;
- (void)deposit:(NSInteger)amount;
- (void)didChange;
@end

// A category declared here and implemented in b-audit.m.
@interface Account (Audit)
- (NSArray<NSString *> *)log;
- (void)note:(NSString *)what;
@end

@interface Savings : Account
@property (nonatomic, assign) double rate;
- (NSInteger)projected;
@end

// An inline function used in two units: §6.7.4 says the definition here
// provides no external one, so each unit that calls it owns a private copy
// and the two must not collide at link time.
static inline NSInteger clampAmount(NSInteger n) {
	if (n < 0) return 0;
	if (n > 1000) return 1000;
	return n;
}
