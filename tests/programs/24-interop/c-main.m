// The program, compiled by objv. It drives both halves and prints what each
// said about the other.

#import "shared.h"

int main(void) {
	@autoreleasepool {
		Widget *w = [Widget widgetWithSerial:7];
		[w setScale:2.5];

		Gadget *g = [Gadget widgetWithSerial:11];
		g.teeth = 9;
		[g setScale:0.5];
		g.label = @"gadget";

		// A method objv compiled, on a class clang compiled.
		printf("%s\n", [[w stamped] UTF8String]);
		// A method clang compiled, on a class objv compiled.
		printf("%s\n", [[g countersigned] UTF8String]);
		// An override reaching super across the line.
		printf("%s\n", [[g describe] UTF8String]);

		// A protocol adopted on one side and checked on the other.
		printf("conforms: %d %d responds(weight): %d %d\n",
		       [w conformsToProtocol:@protocol(Describable)],
		       [g conformsToProtocol:@protocol(Describable)],
		       [w respondsToSelector:@selector(weight)],
		       [g respondsToSelector:@selector(weight)]);

		printf("%s\n", [objvJoin(@[w, g]) UTF8String]);

		// Structs by value, each way.
		Spec s = clangMakeSpec(3, 1.25, 'a');
		printf("%s -> %s\n",
		       [clangRender(s) UTF8String],
		       [clangRender(objvDoubled(s)) UTF8String]);

		// Ivars read through the superclass clang compiled.
		printf("serial=%ld scale=%.2f teeth=%ld\n",
		       (long)[g serial], [g scale], (long)g.teeth);
		printf("class chain: %s <- %s\n",
		       [NSStringFromClass([g class]) UTF8String],
		       [NSStringFromClass([[g class] superclass]) UTF8String]);
	}
	return 0;
}
