// Compiled by objv. It subclasses clang's class, puts a category on it, and
// reads an ivar whose offset clang's object file decides.

#import "shared.h"

@implementation Gadget

- (NSString *)inventory {
	// _serial and _scale are the superclass's, and the superclass was
	// compiled by the other compiler: the offsets come from its object
	// file, through the variables the runtime rewrites at load.
	return [NSString stringWithFormat:@"gadget serial=%ld scale=%.2f teeth=%ld",
	                  (long)_serial, _scale, (long)self.teeth];
}

- (NSString *)describe {
	return [NSString stringWithFormat:@"G<%@>", [super describe]];
}

- (NSInteger)weight { return self.teeth * 2; }

@end

@implementation Widget (ObjvSide)
- (NSString *)stamped {
	return [NSString stringWithFormat:@"objv(%@)", [self describe]];
}
@end

Spec objvDoubled(Spec s) {
	Spec out = s;
	out.serial *= 2;
	out.scale *= 2;
	out.tag[0] = (char)(s.tag[0] + 1);
	return out;
}

NSString *objvJoin(NSArray<id<Describable>> *items) {
	NSMutableArray *parts = [NSMutableArray array];
	for (id<Describable> it in items) {
		NSString *w = [it respondsToSelector:@selector(weight)]
		                ? [NSString stringWithFormat:@"/%ld", (long)[it weight]]
		                : @"";
		[parts addObject:[NSString stringWithFormat:@"%@%@", [it describe], w]];
	}
	return [parts componentsJoinedByString:@" "];
}
