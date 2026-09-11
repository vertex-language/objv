#import <Foundation/Foundation.h>

// A protocol both compilers see, adopted on each side of the line.
@protocol Describable <NSObject>
- (NSString *)describe;
@optional
- (NSInteger)weight;
@end

// Compiled by clang. objv subclasses it, puts a category on it, and reads
// its instance variables.
@interface Widget : NSObject <Describable> {
@protected
	NSInteger _serial;
	double _scale;
}
@property (nonatomic, copy) NSString *label;
+ (instancetype)widgetWithSerial:(NSInteger)serial;
- (NSInteger)serial;
- (double)scale;
- (void)setScale:(double)scale;
@end

// Compiled by objv, used from clang.
@interface Gadget : Widget
@property (nonatomic, assign) NSInteger teeth;
- (NSString *)inventory;
@end

// A category on clang's class, compiled by objv.
@interface Widget (ObjvSide)
- (NSString *)stamped;
@end

// A category on objv's class, compiled by clang.
@interface Gadget (ClangSide)
- (NSString *)countersigned;
@end

// A struct crossing the line by value, in both directions.
typedef struct {
	NSInteger serial;
	double scale;
	char tag[4];
} Spec;

Spec clangMakeSpec(NSInteger serial, double scale, char tag);
NSString *clangRender(Spec s);
Spec objvDoubled(Spec s);
NSString *objvJoin(NSArray<id<Describable>> *items);
