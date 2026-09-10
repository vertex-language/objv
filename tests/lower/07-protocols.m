// Protocol metadata.
//
// A protocol is emitted by every image that mentions it and coalesced by
// the linker down to one, which is why every symbol here is weak and
// hidden: no single image owns a protocol.

@protocol Drawable
- (void)draw;
@optional
- (int)layer;
@end

@protocol Sized <Drawable>
- (int)width;
@end

@interface Widget : NSObject <Sized, NSCopying>
@end

@implementation Widget
- (void)draw { }
- (int)width { return 0; }
- (id)copyWithZone:(void *)zone { return self; }
@end

id which(void) { return (id)@protocol(Drawable); }
// vir: export hidden weak global rw @__OBJC_PROTOCOL_$_Drawable
// vir: section "__DATA,__data"

// The required and the optional methods are separate lists, and each entry
// carries a null implementation: a protocol declares and does not define.
// vir: @__OBJC_$_PROTOCOL_INSTANCE_METHODS_Drawable
// vir: @__OBJC_$_PROTOCOL_INSTANCE_METHODS_OPT_Drawable
// vir: @__OBJC_$_PROTOCOL_METHOD_TYPES_Drawable

// The inherited list, and the entry the runtime finds a protocol by.
// vir: @__OBJC_$_PROTOCOL_REFS_Sized
// vir: export hidden weak global rw @__OBJC_LABEL_PROTOCOL_$_Drawable
// vir: __DATA,__objc_protolist,coalesced,no_dead_strip

// A class's conformance list is the same object in both halves: a class and
// its metaclass conform to the same protocols.
// vir: @__OBJC_CLASS_PROTOCOLS_$_Widget
