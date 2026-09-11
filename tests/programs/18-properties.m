// Properties, in every spelling a real class uses.
//
// A property is two methods and maybe an ivar, and which ones it is depends
// on five attributes that interact: readonly in the header and readwrite in
// the extension, a getter under another name, a setter that copies, an
// atomic one that goes through the runtime rather than through a store, and
// @synthesize pointing at an ivar the program named itself.
//
// The weakSelf dance is here too, because it is the one idiom every block in
// every view controller starts with.
//
// mode: arc

#import <Foundation/Foundation.h>

@interface Document : NSObject

// Readonly to everyone else, readwrite to itself.
@property (nonatomic, copy, readonly) NSString *title;
@property (nonatomic, assign, readonly, getter=isDirty) BOOL dirty;

// A setter that copies, so a mutable string handed in cannot change under it.
@property (nonatomic, copy) NSString *body;

// Atomic, which is the runtime's objc_getProperty rather than a load.
@property (atomic, strong) NSNumber *revision;

// A name of its own for the storage.
@property (nonatomic, assign) NSInteger wordCount;

// A block property, which has to be copy or it is a pointer into a frame.
@property (nonatomic, copy) void (^onChange)(NSString *what);

- (void)setTitle:(NSString *)title;
- (void)touch;
@end

@interface Document ()
// The private half of the two readonly properties above.
@property (nonatomic, copy, readwrite) NSString *title;
@property (nonatomic, assign, readwrite, getter=isDirty) BOOL dirty;
@end

@implementation Document {
	NSInteger _words; // the ivar @synthesize points at
}

@synthesize wordCount = _words;

- (instancetype)init {
	self = [super init];
	if (self) {
		_title = @"untitled";
		_body = @"";
		_revision = @1;
	}
	return self;
}

- (void)setBody:(NSString *)body {
	// The copy is what the attribute promised; the notification is why a
	// hand-written setter exists at all.
	_body = [body copy];
	self.dirty = YES;
	self.wordCount = [[_body componentsSeparatedByString:@" "] count];
	if (self.onChange) self.onChange(@"body");
}

- (void)touch {
	self.revision = @([self.revision integerValue] + 1);
	if (self.onChange) self.onChange(@"revision");
}

- (NSString *)description {
	return [NSString stringWithFormat:@"%@ r%@ (%ld words)%@",
	                  self.title, self.revision, (long)self.wordCount,
	                  self.isDirty ? @" *" : @""];
}

@end

int main(void) {
	@autoreleasepool {
		Document *doc = [[Document alloc] init];

		// The block captures the document weakly, which is the whole reason
		// the idiom exists: the document owns the block.
		__weak Document *weakDoc = doc;
		doc.onChange = ^(NSString *what) {
			Document *strongDoc = weakDoc;
			printf("changed: %s -> %s\n", [what UTF8String],
			       [[strongDoc description] UTF8String]);
		};

		doc.title = @"Report";
		printf("%s dirty=%d\n", [[doc description] UTF8String], doc.isDirty);

		// A mutable string handed to a copy setter, then mutated.
		NSMutableString *live = [NSMutableString stringWithString:@"one two three"];
		doc.body = live;
		[live appendString:@" four"];
		printf("body=%s live=%s\n", [doc.body UTF8String], [live UTF8String]);

		[doc touch];
		[doc touch];

		// The renamed ivar, reached both ways.
		doc.wordCount = 99;
		printf("words=%ld via-getter=%ld\n",
		       (long)doc.wordCount, (long)[doc wordCount]);

		// Dot syntax on a getter with another name, and the setter under
		// the ordinary one.
		printf("isDirty=%d\n", [doc isDirty]);

		// Properties reached through KVC, which uses the *property* name and
		// not the ivar's.
		printf("kvc title=%s revision=%s\n",
		       [[doc valueForKey:@"title"] UTF8String],
		       [[[doc valueForKey:@"revision"] stringValue] UTF8String]);

		printf("final=%s\n", [[doc description] UTF8String]);
	}
	return 0;
}
