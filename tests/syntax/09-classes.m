// §4.1 Classes

@class NSString;
@protocol NSCopying;

// The root class: no superclass, and the attribute that says so
__attribute__((objc_root_class))
@interface RootObject
- (id)init;
@end

@interface Simple : RootObject
@end

@interface WithProtocols : RootObject <NSCopying>
@end

@interface WithMembers : RootObject
- (void)instanceMethod;
+ (void)classMethod;
@end

// An AttributeSpecifierList before @interface, which §4.1 allows and which
// is where every NS_CLASS_AVAILABLE lands
__attribute__((visibility("default")))
@interface Attributed : RootObject
@end

@implementation Simple
@end

@implementation WithMembers
- (void)instanceMethod { }
+ (void)classMethod { }

// §4.6: a function definition inside an @implementation is an ordinary C
// function with access to the class's instance variables
static int helper(int x) { return x * 2; }
@end

// The superclass restated on the implementation
@implementation WithProtocols : RootObject
@end
