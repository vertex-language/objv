// §5.4 Object Type Specifiers, §5.6 Type Qualifiers

@class NSString, NSArray;
@protocol NSCopying, NSCoding;

// The five alternatives of ObjectTypeSpecifier
id anyObject;
Class anyClass;
NSString *named;

// id and Class take a ProtocolReferenceList
id<NSCopying> copying;
id<NSCopying, NSCoding> both;
Class<NSCopying> copyingClass;

// A ClassName takes one too
NSString<NSCopying> *qualifiedName;

// __kindof admits any subclass while keeping the interface
__kindof NSString *kindOf;
__kindof NSString<NSCopying> *kindOfQualified;

// OwnershipQualifier, all four
__strong id strongRef;
__weak id weakRef;
__unsafe_unretained id unsafeRef;
__autoreleasing id *autoreleasingRef;

// NullabilityQualifier, in both spellings §5.6 gives them
NSString *_Nonnull nonnullName;
NSString *_Nullable nullableName;
NSString *_Null_unspecified unspecifiedName;
NSString *__nonnull gnuNonnull;
NSString *__nullable gnuNullable;
NSString *__null_unspecified gnuUnspecified;

// Qualifiers combine, and order does not matter to the grammar
__strong NSString *_Nullable combined;
const __weak id constWeak;

// SEL, IMP and BOOL are typedefs from <objc/objc.h>, not keywords
SEL selector;
IMP implementation;
BOOL flag;

// Protocol is a class, declared with @class under __OBJC__
Protocol *protocolObject;
