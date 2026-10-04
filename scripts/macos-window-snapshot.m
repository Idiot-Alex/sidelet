// Read-only WindowServer/foreground diagnostics; no UI operations.
#import <Cocoa/Cocoa.h>
#import <ApplicationServices/ApplicationServices.h>
int main(int argc,const char **argv) {
    @autoreleasepool {
        NSMutableSet *pids=[NSMutableSet new];
        for (int i=1;i<argc;i++) [pids addObject:@(atoi(argv[i]))];
        NSRunningApplication *front=NSWorkspace.sharedWorkspace.frontmostApplication;
        NSMutableArray *screens=[NSMutableArray new],*windows=[NSMutableArray new];
        for (NSScreen *screen in NSScreen.screens) {
            NSEdgeInsets safe=screen.safeAreaInsets;
            [screens addObject:@{@"frame":NSStringFromRect(screen.frame),@"visibleFrame":NSStringFromRect(screen.visibleFrame),@"safeAreaTop":@(safe.top),@"safeAreaBottom":@(safe.bottom)}];
        }
        NSArray *all=CFBridgingRelease(CGWindowListCopyWindowInfo(kCGWindowListOptionOnScreenOnly|kCGWindowListExcludeDesktopElements,kCGNullWindowID));
        for (NSDictionary *window in all) if ([pids containsObject:window[(id)kCGWindowOwnerPID]])
            [windows addObject:@{@"pid":window[(id)kCGWindowOwnerPID],@"number":window[(id)kCGWindowNumber],@"layer":window[(id)kCGWindowLayer],@"bounds":window[(id)kCGWindowBounds]}];
        NSData *json=[NSJSONSerialization dataWithJSONObject:@{@"foregroundPID":@(front.processIdentifier),@"foregroundBundle":front.bundleIdentifier?:@"",@"screens":screens,@"windows":windows} options:NSJSONWritingPrettyPrinted error:nil];
        puts([[NSString alloc] initWithData:json encoding:NSUTF8StringEncoding].UTF8String);
    }
    return 0;
}
