// Three translation units, linked. Every program before this one was a
// single file, which is the one shape where a compiler can be wrong about
// linkage and never find out.
//
// This file owns the class. The others put a category on it, subclass it,
// and read its globals — so the metadata the runtime walks is assembled out
// of three objects that agreed about it only by following the ABI.

#import "shared.h"

NSString *const AccountDomain = @"account";
int accountsCreated = 0;

// A tentative definition: §6.9.2 says this defines the object, and the
// header's `extern` declares it. One of the two has to become the
// definition and the other the reference, and which is which is settled by
// what the whole link sees.
double accountRate;

@implementation Account

+ (instancetype)named:(NSString *)name {
	Account *a = [[self alloc] init];
	a->_name = [name copy];
	accountsCreated++;
	return a;
}

- (NSString *)name { return _name; }
- (NSInteger)balance { return _balance; }

- (void)deposit:(NSInteger)amount {
	_balance += amount;
	[self didChange];
}

// Declared here, implemented in the category over there. A send to it from
// this file has to reach a method list this file never saw.
- (void)didChange {}

- (NSString *)description {
	return [NSString stringWithFormat:@"%@(%ld)", _name, (long)_balance];
}

@end
