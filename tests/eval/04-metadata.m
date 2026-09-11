// The metadata a class becomes.
//
// None of it is reachable from any call: the runtime finds its work by
// walking sections, which is why every list is emitted whether or not the
// program mentions it and why the sections say no_dead_strip.

@interface Shape : NSObject {
    int _sides;
    NSString *_name;
}
@property (nonatomic) int sides;
- (int)area;
+ (id)make;
@end

@implementation Shape
- (int)area { return _sides; }
+ (id)make { return [[Shape alloc] init]; }
- (int)sides { return _sides; }
- (void)setSides:(int)s { _sides = s; }
@end

@interface Shape (Extra)
- (int)perimeter;
@end

@implementation Shape (Extra)
- (int)perimeter { return 0; }
@end

// The class object and its metaclass, each pointing at a read-only half.
// vir: export global rw @_OBJC_CLASS_$_Shape @objc_class section "__DATA,__objc_data"
// vir: export global rw @_OBJC_METACLASS_$_Shape @objc_class
// vir: internal global ro @__OBJC_CLASS_RO_$_Shape @objc_class_ro
// vir: internal global ro @__OBJC_METACLASS_RO_$_Shape @objc_class_ro

// The lists, entsize first so an older runtime can walk a newer one's.
// vir: @__OBJC_$_INSTANCE_METHODS_Shape
// vir: @__OBJC_$_CLASS_METHODS_Shape
// vir: @__OBJC_$_INSTANCE_VARIABLES_Shape
// vir: @__OBJC_$_PROP_LIST_Shape

// The offset variables, one per instance variable.
// vir: export global rw @_OBJC_IVAR_$_Shape$_sides i32 section "__DATA,__objc_ivar"
// vir: export global rw @_OBJC_IVAR_$_Shape$_name i32

// The category, and the list the runtime scans for it.
// vir: @__OBJC_$_CATEGORY_Shape_$_Extra
// vir: __DATA,__objc_catlist,regular,no_dead_strip

// The strings, in the sections the linker merges across images.
// vir: __TEXT,__objc_classname,cstring_literals
// vir: __TEXT,__objc_methname,cstring_literals
// vir: __TEXT,__objc_methtype,cstring_literals

// The class list and the image info every image carries.
// vir: @_l_OBJC_LABEL_CLASS_$
// vir: __DATA,__objc_classlist,regular,no_dead_strip
// vir: @_L_OBJC_IMAGE_INFO
