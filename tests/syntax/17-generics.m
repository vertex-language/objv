// §5.5 Lightweight Generics

@class NSString, NSNumber;
@protocol NSCopying;

@interface NSObject
@end

@interface NSArray<ObjectType> : NSObject
- (ObjectType)firstObject;
- (void)add:(ObjectType)object;
@end

@interface NSDictionary<KeyType, ObjectType> : NSObject
- (ObjectType)objectForKey:(KeyType)key;
@end

// Variance, and a bound that is a full TypeName
@interface Container<__covariant T : NSObject *> : NSObject
- (T)value;
@end

@interface Sink<__contravariant T> : NSObject
- (void)take:(T)value;
@end

@interface Bounded<T : id<NSCopying>> : NSObject
@end

// TypeArgumentList in every position that takes a type
NSArray<NSString *> *strings;
NSArray<NSArray<NSString *> *> *nested;
NSDictionary<NSString *, NSNumber *> *map;

// The two lists together: type arguments, then protocols
NSArray<NSString *> <NSCopying> *both;

// The place §6.6 warns about: two lists closed by one token
NSArray<id<NSCopying>> *qualifiedElements;
NSDictionary<NSString *, NSArray<id<NSCopying>> *> *deeplyNested;

// A generic superclass, with the class's own protocol list after it
@interface StringArray : NSArray<NSString *> <NSCopying>
@end

// The implementation takes neither list: generics are erased
@implementation NSArray
- (id)firstObject { return nil; }
- (void)add:(id)object { (void)object; }
@end

@implementation StringArray
@end
