// §4.3 Protocols

@protocol Forward;
@protocol ForwardA, ForwardB;

@protocol Empty
@end

@protocol Simple
- (void)required;
@end

@protocol Inherited <Simple>
- (void)more;
@end

@protocol Multiple <Simple, Inherited>
@end

// ProtocolSection: @required and @optional, and the default before either
@protocol Sections
- (void)defaultsToRequired;
@required
- (void)explicitlyRequired;
+ (id)requiredClassMethod;
@optional
- (void)optionalMethod;
@property (nonatomic, readonly) int optionalProperty;
@required
- (void)requiredAgain;
@end

// An attribute list before @protocol
__attribute__((deprecated))
@protocol Deprecated
@end

// A protocol may declare properties and C declarations too
@protocol WithEverything <Simple>
@property (nonatomic, copy) id name;
- (void)method;
+ (void)classMethod;
@end
