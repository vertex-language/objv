// §4.7 Methods

@class NSString;
@protocol NSCopying;
typedef long NSInteger;

@interface Methods
// MethodKind: - is an instance method, + a class method
- (void)unary;
+ (void)classUnary;

// KeywordDeclarator, one and many
- (void)setValue:(int)value;
- (void)setValue:(int)value forKey:(NSString *)key;
- (id)initWithA:(int)a b:(int)b c:(int)c;

// instancetype, valid only as a return type
- (instancetype)init;
+ (instancetype)methods;

// A MethodType with only distributed-object qualifiers, and no type at all
- (oneway)shutdown;
- (bycopy NSString *)copiedString;
- (oneway void)release;
- (void)compute:(in int)input result:(out int *)result;
- (void)exchange:(inout id)object;
- (void)byref:(byref id)object;

// The underscore-free nullability spellings, valid only in a MethodType
- (nullable NSString *)maybeName;
- (nonnull NSString *)definitelyName;
- (null_unspecified NSString *)unknownName;
- (void)takeMaybe:(nullable id)maybe;

// Any reserved word may be a selector piece
- (void)default:(int)x;
- (void)if:(int)x else:(int)y;
- (int)int;
- (void)copy:(id)x;

// A KeywordDeclarator may omit its Selector, so the second piece is
// nameless and the selector is b::
- (void)b:(int)x :(int)y;

// MethodParameterSuffix: the C-style trailing parameters
- (void)format:(NSString *)fmt, ...;
- (void)format:(NSString *)fmt arguments:(int)first, ...;

// Attributes in all three positions §4.7 admits
- (void)__attribute__((deprecated)) attributedBeforeSelector;
- (void)takeUnused:(int)__attribute__((unused))x;
- (instancetype)initWithCapacity:(NSInteger)cap __attribute__((objc_designated_initializer));

// Generic and protocol-qualified parameter types
- (void)takeCopying:(id<NSCopying>)object;
- (void)takeKindOf:(__kindof NSString *)string;
@end

@implementation Methods
- (void)unary { }
+ (void)classUnary { }
- (void)setValue:(int)value { (void)value; }
- (void)setValue:(int)value forKey:(NSString *)key { (void)value; (void)key; }
- (id)initWithA:(int)a b:(int)b c:(int)c { (void)a; (void)b; (void)c; return self; }
- (instancetype)init { return self; }
+ (instancetype)methods { return nil; }
- (oneway)shutdown { }
- (NSString *)copiedString { return nil; }
- (oneway void)release { }
- (void)compute:(in int)input result:(out int *)result { (void)input; (void)result; }
- (void)exchange:(inout id)object { (void)object; }
- (void)byref:(byref id)object { (void)object; }
- (NSString *)maybeName { return nil; }
- (NSString *)definitelyName { return nil; }
- (NSString *)unknownName { return nil; }
- (void)takeMaybe:(id)maybe { (void)maybe; }
- (void)default:(int)x { (void)x; }
- (void)if:(int)x else:(int)y { (void)x; (void)y; }
- (int)int { return 0; }
- (void)copy:(id)x { (void)x; }
- (void)b:(int)x :(int)y { (void)x; (void)y; }
- (void)format:(NSString *)fmt, ... { (void)fmt; }
- (void)format:(NSString *)fmt arguments:(int)first, ... { (void)fmt; (void)first; }
- (void)attributedBeforeSelector { }
- (void)takeUnused:(int)x { (void)x; }
- (instancetype)initWithCapacity:(NSInteger)cap { (void)cap; return self; }
- (void)takeCopying:(id<NSCopying>)object { (void)object; }
- (void)takeKindOf:(__kindof NSString *)string { (void)string; }
@end
