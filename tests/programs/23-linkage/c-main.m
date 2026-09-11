// The subclass, and the program. It reads the globals a-model.m defines,
// sends to methods b-audit.m implemented, and inherits ivars declared in a
// header — which under the non-fragile ABI means loading an offset variable
// that another object file wrote.

#import "shared.h"

@implementation Savings

- (NSInteger)projected {
	// _balance is the superclass's ivar, reached from another unit.
	return _balance + (NSInteger)(_balance * self.rate);
}

- (void)didChange {
	// super reaches the *category's* implementation, which is the one the
	// runtime attached rather than the one the class was compiled with.
	[super didChange];
	[self note:@"savings"];
}

@end

int main(void) {
	@autoreleasepool {
		accountRate = 0.05;

		Account *a = [Account named:@"ada"];
		[a deposit:clampAmount(250)];
		[a deposit:clampAmount(5000)];

		Savings *s = [Savings named:@"grace"];
		s.rate = accountRate;
		[s deposit:clampAmount(-5)];
		[s deposit:clampAmount(400)];

		printf("domain=%s created=%d rate=%.2f\n",
		       [AccountDomain UTF8String], accountsCreated, accountRate);
		printf("%s | %s projected=%ld\n",
		       [[a description] UTF8String], [[s description] UTF8String],
		       (long)[s projected]);
		printf("a log: %s\n", [[[a log] componentsJoinedByString:@","] UTF8String]);
		printf("s log: %s\n", [[[s log] componentsJoinedByString:@","] UTF8String]);
		printf("kinds: %d %d\n",
		       [s isKindOfClass:[Account class]], [a isKindOfClass:[Savings class]]);
	}
	return 0;
}
