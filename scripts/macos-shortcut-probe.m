// Registration probe only. This program never synthesizes or posts key events.
#import <Cocoa/Cocoa.h>
#import <Carbon/Carbon.h>
#include <string.h>
#include <unistd.h>

int main(int argc, const char **argv) {
    if (argc < 2 || (strcmp(argv[1],"free") && strcmp(argv[1],"occupied") && strcmp(argv[1],"hold"))) return 2;
    @autoreleasepool {
        [NSApplication sharedApplication];
        EventHotKeyRef ref = NULL;
        EventHotKeyID id = {'SLtp',1};
        BOOL quickAdd=argc>3 && !strcmp(argv[3],"quick-add");
        UInt32 code=quickAdd?kVK_Space:kVK_ANSI_T, modifiers=quickAdd?(controlKey|shiftKey):(controlKey|optionKey);
        OSStatus result = RegisterEventHotKey(code,modifiers,id,GetApplicationEventTarget(),kEventHotKeyExclusive,&ref);
        printf("shortcut-probe pid=%d code=%d modifiers=%u exclusive=true status=%d\n",getpid(),code,modifiers,(int)result);
        fflush(stdout);
        BOOL passed = !strcmp(argv[1],"occupied") ? result==eventHotKeyExistsErr : result==noErr;
        if (ref) {
            if (!strcmp(argv[1],"hold")) {
                unsigned seconds = argc>=3 ? (unsigned)atoi(argv[2]) : 10;
                if (!seconds || seconds>30) { UnregisterEventHotKey(ref); return 2; }
                sleep(seconds);
            }
            OSStatus release = UnregisterEventHotKey(ref);
            printf("shortcut-probe unregister status=%d\n",(int)release);
            passed = passed && release==noErr;
        }
        printf("%s exclusive registration expectation=%s (no key delivery tested)\n",passed?"PASS":"FAIL",argv[1]);
        return passed ? 0 : 1;
    }
}
