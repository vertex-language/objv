// Compiled by clang in both builds. Everything here is metadata objv's half
// has to agree with: a class layout, an ivar offset, a method list, a
// protocol, and a struct convention.

#import "shared.h"

@implementation Widget

+ (instancetype)widgetWithSerial:(NSInteger)serial {
	Widget *w = [[self alloc] init];
	w->_serial = serial;
	w->_scale = 1.0;
	w.label = @"widget";
	return w;
}

- (NSInteger)serial { return _serial; }
- (double)scale { return _scale; }
- (void)setScale:(double)scale { _scale = scale; }

- (NSString *)describe {
	return [NSString stringWithFormat:@"%@#%ld@%.2f", self.label, (long)_serial, _scale];
}

@end

@implementation Gadget (ClangSide)
- (NSString *)countersigned {
	// Reaches a method objv compiled, on a class objv compiled.
	return [NSString stringWithFormat:@"clang(%@)", [self inventory]];
}
@end

Spec clangMakeSpec(NSInteger serial, double scale, char tag) {
	Spec s = {.serial = serial, .scale = scale};
	s.tag[0] = tag;
	s.tag[1] = (char)(tag + 1);
	s.tag[2] = 0;
	return s;
}

NSString *clangRender(Spec s) {
	return [NSString stringWithFormat:@"[%ld %.2f %s]", (long)s.serial, s.scale, s.tag];
}
