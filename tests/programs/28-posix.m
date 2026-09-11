// The C library under Objective-C: the headers every program includes and
// nobody thinks about.
//
// <errno.h> is a macro over a function returning a pointer. <time.h> is a
// struct with a dozen fields and a formatter. <stdatomic.h> is _Atomic and
// generic macros. <setjmp.h> is a function that returns twice. <unistd.h>
// and <sys/stat.h> are a hundred attributed prototypes each. None of it is
// Objective-C and all of it is in every Objective-C program.

#import <Foundation/Foundation.h>
#include <errno.h>
#include <fcntl.h>
#include <setjmp.h>
#include <stdatomic.h>
#include <string.h>
#include <time.h>
#include <unistd.h>
#include <sys/stat.h>

// A counter the compiler may not reorder around, which is what _Atomic is
// for and what the generic macros in <stdatomic.h> expand to.
static _Atomic(long) gTicks;

@interface Clock : NSObject
+ (NSString *)stamp:(time_t)t;
+ (long)bump:(long)by;
@end

@implementation Clock

+ (NSString *)stamp:(time_t)t {
	struct tm parts;
	gmtime_r(&t, &parts);
	char buf[64];
	strftime(buf, sizeof buf, "%Y-%m-%dT%H:%M:%SZ", &parts);
	return [NSString stringWithUTF8String:buf];
}

+ (long)bump:(long)by {
	return atomic_fetch_add(&gTicks, by);
}

@end

// setjmp/longjmp: a function that returns twice, which the compiler has to
// know about or the locals it kept in registers are wrong afterwards.
static jmp_buf gJump;

static int divide(int a, int b) {
	if (b == 0) longjmp(gJump, 42);
	return a / b;
}

int main(void) {
	@autoreleasepool {
		// A fixed instant, so the output is a fact about the formatter.
		printf("epoch=%s\n", [[Clock stamp:0] UTF8String]);
		printf("y2k=%s\n", [[Clock stamp:946684800] UTF8String]);

		// tm's fields, which are a struct layout the library and the
		// program both have to agree about.
		struct tm parts;
		time_t when = 1234567890;
		gmtime_r(&when, &parts);
		printf("parts: %d-%02d-%02d %02d:%02d:%02d wday=%d yday=%d\n",
		       parts.tm_year + 1900, parts.tm_mon + 1, parts.tm_mday,
		       parts.tm_hour, parts.tm_min, parts.tm_sec,
		       parts.tm_wday, parts.tm_yday);
		printf("roundtrip=%d\n", (int)(timegm(&parts) == when));

		// errno is `(*__error())`, a macro over a call.
		errno = 0;
		int fd = open("/definitely/not/here", O_RDONLY);
		printf("open=%d errno=%d enoent=%d msg=%s\n",
		       fd, errno, errno == ENOENT, strerror(ENOENT));

		// stat on something that is always there, with only stable fields
		// printed.
		struct stat st;
		int ok = stat("/usr/lib", &st) == 0;
		printf("stat=%d dir=%d size>0=%d\n",
		       ok, ok && S_ISDIR(st.st_mode), ok && st.st_size > 0);

		// Atomics, through the generic macros.
		atomic_store(&gTicks, 10);
		printf("atomic: %ld %ld %ld\n",
		       [Clock bump:5], [Clock bump:5], atomic_load(&gTicks));
		long expected = 20;
		printf("cas=%d now=%ld\n",
		       atomic_compare_exchange_strong(&gTicks, &expected, 99),
		       atomic_load(&gTicks));
		printf("lock free=%d\n", atomic_is_lock_free(&gTicks));

		// setjmp returns 0 the first time and the longjmp's value after.
		int caught = setjmp(gJump);
		if (caught == 0) {
			printf("div=%d\n", divide(84, 2));
			printf("never=%d\n", divide(1, 0));
		} else {
			printf("longjmp=%d\n", caught);
		}

		// String and memory functions, which are what §E's bulk verbs
		// become.
		char buf[32];
		memset(buf, 'x', sizeof buf);
		memcpy(buf, "objv", 4);
		buf[31] = 0;
		printf("buf=%.8s len=%zu cmp=%d\n", buf, strlen("objective"),
		       memcmp("abc", "abd", 3) < 0);

		char moved[16] = "0123456789";
		memmove(moved + 2, moved, 8);
		printf("moved=%s\n", moved);

		// sysconf and getpid exist; only their shape is printed.
		printf("page>0=%d pid>0=%d\n",
		       sysconf(_SC_PAGESIZE) > 0, getpid() > 0);
	}
	return 0;
}
