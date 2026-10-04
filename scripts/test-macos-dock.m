// Exercise AppKit policy changes inside this fixture's own application only.
#import <AppKit/AppKit.h>
#import "../internal/platform/settings_darwin.m"
static int failures;
static void check(BOOL passed, NSString *name) {
    printf("%s %s\n", passed ? "PASS" : "FAIL", name.UTF8String);
    if (!passed) failures++;
}
@interface DockTestDelegate : NSObject <NSApplicationDelegate>
@property(strong) NSWindow *control;
@property(strong) NSPanel *quick;
@end
@implementation DockTestDelegate
- (void)applicationDidFinishLaunching:(NSNotification *)notification {
    self.control=[[NSWindow alloc] initWithContentRect:NSMakeRect(220,220,420,180) styleMask:NSWindowStyleMaskTitled backing:NSBackingStoreBuffered defer:NO];
    self.control.releasedWhenClosed=NO;
    self.control.title=@"Sidelet Dock regression fixture";
    self.quick=[[NSPanel alloc] initWithContentRect:NSMakeRect(220,220,120,100) styleMask:NSWindowStyleMaskBorderless | NSWindowStyleMaskNonactivatingPanel backing:NSBackingStoreBuffered defer:NO];
    self.quick.releasedWhenClosed=NO;
    BOOL wasActive=NSApp.active;
    check(SLSetDockVisible(1,(__bridge void *)self.control) && NSApp.activationPolicy==NSApplicationActivationPolicyRegular,@"enable Dock policy");
    check(!self.control.visible && !self.quick.visible,@"background promotion does not reveal windows");
    check(NSApp.active==wasActive,@"background promotion does not activate application");
    [NSApp activateIgnoringOtherApps:YES];
    [self.control makeKeyAndOrderFront:nil];
    dispatch_after(dispatch_time(DISPATCH_TIME_NOW,300*NSEC_PER_MSEC),dispatch_get_main_queue(),^{
        check(self.control.visible && self.control.keyWindow,@"fixture main window is visible and focused");
        check(SLSetDockVisible(0,(__bridge void *)self.control) && NSApp.activationPolicy==NSApplicationActivationPolicyAccessory,@"hide Dock policy");
        dispatch_after(dispatch_time(DISPATCH_TIME_NOW,300*NSEC_PER_MSEC),dispatch_get_main_queue(),^{
            check(self.control.visible && self.control.keyWindow && NSApp.active,@"hiding Dock preserves the open settings window and focus");
            check(!self.quick.visible,@"hiding Dock does not reveal a hidden card");
            check(SLSetDockVisible(1,(__bridge void *)self.control) && self.control.visible,@"re-enable Dock with the main window open");
            [self.control orderOut:nil];
            SLSetDockVisible(0,(__bridge void *)self.control);
            SLSetDockVisible(1,(__bridge void *)self.control);
            check(!self.control.visible && !self.quick.visible,@"policy changes preserve intentionally hidden windows");
            [self.control close]; [self.quick close];
            printf("Dock assertions: %d failures\n",failures);
            exit(failures ? 1 : 0);
        });
    });
    dispatch_after(dispatch_time(DISPATCH_TIME_NOW,10*NSEC_PER_SEC),dispatch_get_main_queue(),^{fprintf(stderr,"FAIL Dock test timed out\n");exit(1);});
}
@end
int main(void) {
    @autoreleasepool {
        NSApplication *app=NSApplication.sharedApplication;
        [app setActivationPolicy:NSApplicationActivationPolicyAccessory];
        DockTestDelegate *delegate=[DockTestDelegate new];app.delegate=delegate;
        [app run];
    }
    return 1;
}
