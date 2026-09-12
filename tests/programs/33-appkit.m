// A real application's view hierarchy, built and inspected without ever
// showing it.
//
// AppKit is where an Objective-C program stops being Objective-C exercises
// and starts being an application: a window, a view tree with autoresizing
// masks, controls wired to selectors, a delegate that conforms to three
// protocols at once, and a hundred enumerations whose values have to be the
// same numbers the framework compiled against.
//
// The bug this file was written for is one line of AppKit's own header.
// NSApp is declared *inside* the @interface it belongs to:
//
//	@interface NSApplication : NSResponder <…>
//	APPKIT_EXTERN __kindof NSApplication *NSApp;
//
// A class body holds methods, properties and instance variables (§4); a C
// declaration written in one is at file scope wherever it stands. Lowering
// collected those from an @implementation and not from an @interface, so
// `[NSApp activate…]` -- which is in the main of every AppKit program --
// reached lowering with nothing to refer to.
//
// Nothing here calls -run or orders a window on screen: the point is the
// tree, not the event loop.
//
// frameworks: Cocoa

#import <Cocoa/Cocoa.h>

// The same shape AppKit uses, in this unit's own class: a C declaration
// between the class's first line and its first property.
@interface Registry : NSObject
extern NSInteger gRegistryBuilds;
extern NSString *const kRegistryName;
@property (strong) NSMutableArray<NSView *> *views;
- (NSInteger)count;
@end

NSInteger gRegistryBuilds = 0;
NSString *const kRegistryName = @"registry";

@implementation Registry
// And the other spelling, which already worked: a static between two
// methods, which is how a class gets one variable rather than one each.
static NSInteger sInstances = 0;
- (instancetype)init {
    if ((self = [super init])) {
        _views = [NSMutableArray array];
        sInstances++;
        gRegistryBuilds++;
    }
    return self;
}
- (NSInteger)count { return self.views.count; }
+ (NSInteger)instances { return sInstances; }
@end

@protocol Sizing <NSObject>
extern const CGFloat kHairline;   // and inside a @protocol
- (NSRect)boundsForWidth:(CGFloat)w;
@end

const CGFloat kHairline = 1.0;

@interface Builder : NSObject <NSApplicationDelegate, NSTextFieldDelegate, Sizing>
@property (strong) NSWindow *window;
@property (strong) NSTextField *urlField;
@property (strong) NSButton *backButton;
@property (strong) NSSegmentedControl *tabBar;
@property (strong) Registry *registry;
@property (assign) NSInteger tabHits;
@end

@implementation Builder

- (NSRect)boundsForWidth:(CGFloat)w { return NSMakeRect(0, 0, w, w * 0.75); }

- (void)build {
    self.registry = [[Registry alloc] init];

    NSRect frame = NSMakeRect(0, 0, 1024, 720);
    NSWindowStyleMask mask = NSWindowStyleMaskTitled |
                             NSWindowStyleMaskClosable |
                             NSWindowStyleMaskMiniaturizable |
                             NSWindowStyleMaskResizable |
                             NSWindowStyleMaskFullSizeContentView;
    self.window = [[NSWindow alloc] initWithContentRect:frame
                                              styleMask:mask
                                                backing:NSBackingStoreBuffered
                                                  defer:NO];
    [self.window setTitle:@"Mini Browser"];
    self.window.titlebarAppearsTransparent = YES;
    self.window.titleVisibility = NSWindowTitleHidden;

    NSView *root = self.window.contentView;
    CGFloat topBarHeight = 82.0;
    NSRect bounds = root.bounds;

    NSVisualEffectView *header = [[NSVisualEffectView alloc] initWithFrame:
        NSMakeRect(0, bounds.size.height - topBarHeight, bounds.size.width, topBarHeight)];
    header.material = NSVisualEffectMaterialHeaderView;
    header.blendingMode = NSVisualEffectBlendingModeWithinWindow;
    header.state = NSVisualEffectStateActive;
    header.autoresizingMask = NSViewWidthSizable | NSViewMinYMargin;
    [root addSubview:header];
    [self.registry.views addObject:header];

    NSBox *hairline = [[NSBox alloc] initWithFrame:NSMakeRect(0, 0, bounds.size.width, kHairline)];
    hairline.boxType = NSBoxCustom;
    hairline.borderColor = [NSColor separatorColor];
    hairline.borderWidth = kHairline;
    hairline.autoresizingMask = NSViewWidthSizable;
    [header addSubview:hairline];

    self.tabBar = [NSSegmentedControl segmentedControlWithLabels:@[@"Apple", @"GitHub", @"Wikipedia"]
                                                    trackingMode:NSSegmentSwitchTrackingSelectOne
                                                          target:self
                                                          action:@selector(tabChanged:)];
    self.tabBar.frame = NSMakeRect(88, 46, 300, 24);
    self.tabBar.segmentStyle = NSSegmentStyleTexturedRounded;
    self.tabBar.selectedSegment = 0;
    [header addSubview:self.tabBar];

    NSImage *backIcon = [NSImage imageWithSystemSymbolName:@"chevron.left"
                                  accessibilityDescription:@"Back"];
    self.backButton = [NSButton buttonWithImage:backIcon target:self action:@selector(goBack:)];
    self.backButton.frame = NSMakeRect(14, 10, 30, 26);
    self.backButton.bezelStyle = NSBezelStyleTexturedRounded;
    self.backButton.enabled = NO;
    [header addSubview:self.backButton];

    CGFloat urlX = 120.0;
    self.urlField = [[NSTextField alloc] initWithFrame:
        NSMakeRect(urlX, 9, bounds.size.width - urlX - 16.0, 28)];
    self.urlField.placeholderString = @"Search or enter website address";
    self.urlField.target = self;
    self.urlField.action = @selector(urlEntered:);
    self.urlField.font = [NSFont systemFontOfSize:13];
    self.urlField.focusRingType = NSFocusRingTypeExterior;
    self.urlField.wantsLayer = YES;
    self.urlField.layer.cornerRadius = 7.0;
    self.urlField.layer.masksToBounds = YES;
    self.urlField.autoresizingMask = NSViewWidthSizable;
    [header addSubview:self.urlField];
    [self.registry.views addObject:self.urlField];
}

// The pure half of a browser: the thing that turns what was typed into a URL.
- (NSString *)normalize:(NSString *)input {
    NSString *t = [input stringByTrimmingCharactersInSet:
                   [NSCharacterSet whitespaceAndNewlineCharacterSet]];
    if (t.length == 0) return nil;
    if (![t hasPrefix:@"http://"] && ![t hasPrefix:@"https://"]) {
        t = [NSString stringWithFormat:@"https://%@", t];
    }
    return t;
}

- (void)urlEntered:(id)sender {
    NSString *t = [self normalize:self.urlField.stringValue];
    if (t) self.urlField.stringValue = t;
}

- (void)goBack:(id)sender { }

- (void)tabChanged:(NSSegmentedControl *)sender {
    self.tabHits++;
    switch (sender.selectedSegment) {
    case 0: self.urlField.stringValue = @"https://www.apple.com"; break;
    case 1: self.urlField.stringValue = @"https://github.com"; break;
    case 2: self.urlField.stringValue = @"https://en.wikipedia.org"; break;
    }
}

@end

int main(int argc, const char *argv[]) {
    @autoreleasepool {
        NSApplication *app = [NSApplication sharedApplication];
        [app setActivationPolicy:NSApplicationActivationPolicyProhibited];

        // The declaration that sent this file here. NSApp is the shared
        // application the line above created.
        printf("nsapp %d %d\n", (int)(NSApp == app), (int)(NSApp != nil));

        Builder *b = [[Builder alloc] init];
        [app setDelegate:b];
        printf("delegate %d\n", (int)(app.delegate == b));

        [b build];

        NSView *root = b.window.contentView;
        printf("window %s %g x %g\n", b.window.title.UTF8String,
               b.window.frame.size.width, b.window.frame.size.height);
        printf("titlebar %d %ld\n", (int)b.window.titlebarAppearsTransparent,
               (long)b.window.titleVisibility);
        printf("root subviews %lu\n", (unsigned long)root.subviews.count);

        NSVisualEffectView *hdr = nil;
        for (NSView *v in root.subviews) {
            if ([v isKindOfClass:NSVisualEffectView.class]) hdr = (NSVisualEffectView *)v;
        }
        printf("header %g x %g material %ld blending %ld state %ld subviews %lu\n",
               hdr.frame.size.width, hdr.frame.size.height,
               (long)hdr.material, (long)hdr.blendingMode, (long)hdr.state,
               (unsigned long)hdr.subviews.count);

        printf("tabs %ld sel %ld style %ld labels", (long)b.tabBar.segmentCount,
               (long)b.tabBar.selectedSegment, (long)b.tabBar.segmentStyle);
        for (NSInteger i = 0; i < b.tabBar.segmentCount; i++) {
            printf(" %s", [b.tabBar labelForSegment:i].UTF8String);
        }
        printf("\n");
        printf("tabbar %g %g %g %g\n", b.tabBar.frame.origin.x, b.tabBar.frame.origin.y,
               b.tabBar.frame.size.width, b.tabBar.frame.size.height);
        printf("back %g %g %g %g enabled %d bezel %ld\n",
               b.backButton.frame.origin.x, b.backButton.frame.origin.y,
               b.backButton.frame.size.width, b.backButton.frame.size.height,
               (int)b.backButton.enabled, (long)b.backButton.bezelStyle);
        printf("url %g %g %g %g radius %g masks %d font %g\n",
               b.urlField.frame.origin.x, b.urlField.frame.origin.y,
               b.urlField.frame.size.width, b.urlField.frame.size.height,
               b.urlField.layer.cornerRadius, (int)b.urlField.layer.masksToBounds,
               b.urlField.font.pointSize);
        printf("placeholder %s\n", b.urlField.placeholderString.UTF8String);

        // The C declarations written inside a class body and a protocol.
        printf("globals %ld %ld %s %g\n", (long)gRegistryBuilds,
               (long)[Registry instances], kRegistryName.UTF8String, kHairline);
        printf("registry %ld\n", (long)[b.registry count]);

        // Conformance, which is what an application delegate is.
        printf("conforms %d %d %d\n",
               (int)[b conformsToProtocol:@protocol(NSApplicationDelegate)],
               (int)[b conformsToProtocol:@protocol(NSTextFieldDelegate)],
               (int)[b conformsToProtocol:@protocol(Sizing)]);
        NSRect sized = [b boundsForWidth:400];
        printf("sizing %g %g\n", sized.size.width, sized.size.height);

        for (NSString *s in @[@"apple.com", @"  github.com  ", @"https://x.dev",
                              @"http://y.dev", @"", @"   "]) {
            NSString *t = [b normalize:s];
            printf("norm(%s) -> %s\n", s.UTF8String, t ? t.UTF8String : "(nil)");
        }

        // The actions, driven without a click.
        for (NSInteger i = 0; i < 3; i++) {
            b.tabBar.selectedSegment = i;
            [b tabChanged:b.tabBar];
            printf("tab %ld -> %s\n", (long)i, b.urlField.stringValue.UTF8String);
        }
        b.urlField.stringValue = @"  example.org ";
        [b urlEntered:nil];
        printf("entered %s hits %ld\n", b.urlField.stringValue.UTF8String, (long)b.tabHits);

        // And the send the whole file is here for.
        [NSApp setActivationPolicy:NSApplicationActivationPolicyProhibited];
        printf("policy %ld\n", (long)NSApp.activationPolicy);
    }
    return 0;
}
