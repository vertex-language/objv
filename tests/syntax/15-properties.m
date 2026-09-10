// §4.8 Properties

@class NSString, NSArray;
@protocol NSCopying;

@interface Properties
// No attribute list at all, and an empty one — which §4.8 permits
@property int bare;
@property () NSString *empty;

// Every PropertyAttributeName of §4.8's closed set
@property (atomic) int atomicProp;
@property (nonatomic) int nonatomicProp;
@property (readonly) int readonlyProp;
@property (readwrite) int readwriteProp;
@property (assign) int assignProp;
@property (retain) id retainProp;
@property (copy) id copyProp;
@property (strong) id strongProp;
@property (weak) id weakProp;
@property (unsafe_unretained) id unsafeProp;
@property (nullable) id nullableProp;
@property (nonnull) id nonnullProp;
@property (null_resettable) id resettableProp;
@property (null_unspecified) id unspecifiedProp;
@property (class, readonly) id classProp;
@property (direct) int directProp;

// getter and setter, with the setter's trailing colon
@property (getter=isEnabled) int enabled;
@property (setter=setEnabledFlag:) int flag;
@property (nonatomic, getter=isHidden, setter=setHidden:) int hidden;

// A combination, and several declarators in one declaration
@property (nonatomic, copy, readonly) NSString *name;
@property (nonatomic, copy) NSString *first, *last;

// Qualified and generic property types
@property (nonatomic, copy) id<NSCopying> copying;
@property (nonatomic, copy) NSArray<NSString *> *strings;
@property (nonatomic, strong) __kindof NSString *kindOfString;
@property (nonatomic, copy) void (^handler)(NSString *);

// An attribute after the declarator
@property (nonatomic) int deprecated __attribute__((deprecated));
@end

@implementation Properties
// PropertyImplementation
@synthesize bare;
@synthesize name = _name;
@synthesize first = _first, last = _last;
@dynamic classProp;
@dynamic enabled, flag;
@end
