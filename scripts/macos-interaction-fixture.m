#import <Cocoa/Cocoa.h>
#import <ApplicationServices/ApplicationServices.h>

static FILE *fixtureLog;
static NSTextField *status;
static NSInteger clicks, wheels;
static int sideletPID;
static unsigned long evidenceSequence;
static BOOL manualSession;
static NSWindow *fixtureWindow;
static NSTextField *fixtureInput;
static id fixtureEventMonitor;
static NSMutableDictionary *focusFields(void) {
    NSResponder *responder=fixtureWindow.firstResponder;
    BOOL inputFocused=responder==fixtureInput || ([responder isKindOfClass:NSTextView.class] && ((NSTextView *)responder).delegate==(id)fixtureInput);
    return [@{@"fixturePID":@(getpid()),@"sideletPID":@(sideletPID),
        @"foregroundPID":@(NSWorkspace.sharedWorkspace.frontmostApplication.processIdentifier),@"appActive":@(NSApp.active),
        @"windowKey":@(fixtureWindow.isKeyWindow),@"inputFocused":@(inputFocused),@"input":fixtureInput.stringValue?:@"",
        @"firstResponderClass":responder?NSStringFromClass(responder.class):@""} mutableCopy];
}
static NSMutableDictionary *eventFields(NSEvent *event) {
    NSMutableDictionary *record=focusFields();
    NSPoint screen=[event.window convertPointToScreen:event.locationInWindow];
    record[@"screenX"]=@(screen.x);
    record[@"screenY"]=@(NSMaxY(NSScreen.screens.firstObject.frame)-screen.y);
    return record;
}
static void evidence(NSString *type, NSDictionary *value) {
    NSMutableDictionary *record=[value mutableCopy];
    record[@"type"]=type; record[@"sequence"]=@(++evidenceSequence);
    record[@"uptime"]=@(NSProcessInfo.processInfo.systemUptime);
    record[@"unixTime"]=@(NSDate.date.timeIntervalSince1970);
    NSData *data=[NSJSONSerialization dataWithJSONObject:record options:0 error:nil];
    fprintf(fixtureLog,"evidence=%s\n",[[NSString alloc] initWithData:data encoding:NSUTF8StringEncoding].UTF8String); fflush(fixtureLog);
}
static void recordClick(NSEvent *event) {
    clicks++;
    fprintf(fixtureLog,"mouseDown count=%ld windowX=%.1f windowY=%.1f\n",(long)clicks,event.locationInWindow.x,event.locationInWindow.y);
    fflush(fixtureLog);
    status.stringValue=[NSString stringWithFormat:@"Underlying clicks: %ld · wheel events: %ld",(long)clicks,(long)wheels];
}
@interface TestSurface : NSView
@end
@implementation TestSurface
- (BOOL)isFlipped { return YES; }
- (void)drawRect:(NSRect)rect { [[NSColor colorWithRed:.94 green:.96 blue:.98 alpha:1] setFill]; NSRectFill(rect); }
- (void)mouseDown:(NSEvent *)event { recordClick(event); }
@end
@interface ScrollDocument : TestSurface
@end
@implementation ScrollDocument
- (void)drawRect:(NSRect)rect {
    [super drawRect:rect];
    NSDictionary *attrs=@{NSFontAttributeName:[NSFont systemFontOfSize:16]};
    for (int y=10;y<NSHeight(self.bounds);y+=50)
        [[NSString stringWithFormat:@"Disposable underlying scroll document · row %d",y/50+1] drawAtPoint:NSMakePoint(20,y) withAttributes:attrs];
}
@end
@interface FixtureScroll : NSScrollView
@property(strong) id boundsObserver;
@property(strong) NSMutableDictionary *pendingScroll;
@end
@implementation FixtureScroll
- (instancetype)initWithFrame:(NSRect)frame {
    self=[super initWithFrame:frame];
    if (self) {
        self.contentView.postsBoundsChangedNotifications=YES;
        __weak FixtureScroll *weakSelf=self;
        self.boundsObserver=[NSNotificationCenter.defaultCenter addObserverForName:NSViewBoundsDidChangeNotification object:self.contentView queue:nil usingBlock:^(NSNotification *note) {
            FixtureScroll *scroll=weakSelf;
            NSMutableDictionary *record=scroll.pendingScroll;
            if (record && fabs(scroll.documentVisibleRect.origin.y-[record[@"beforeY"] doubleValue])>.01) {
                record[@"afterY"]=@(scroll.documentVisibleRect.origin.y);
                evidence(@"scroll",record);
                scroll.pendingScroll=nil;
            }
        }];
    }
    return self;
}
- (void)dealloc { if (self.boundsObserver) [NSNotificationCenter.defaultCenter removeObserver:self.boundsObserver]; }
- (void)scrollWheel:(NSEvent *)event {
    wheels++;
    double before=self.documentVisibleRect.origin.y;
    NSMutableDictionary *record=eventFields(event);
    record[@"beforeY"]=@(before); record[@"eventUnixTime"]=@(NSDate.date.timeIntervalSince1970);
    self.pendingScroll=record;
    evidence(@"scroll-request",record);
    [super scrollWheel:event];
    fprintf(fixtureLog,"scroll count=%ld delta=%.1f visibleY=%.1f\n",(long)wheels,event.scrollingDeltaY,self.documentVisibleRect.origin.y); fflush(fixtureLog);
    status.stringValue=[NSString stringWithFormat:@"Underlying clicks: %ld · wheel events: %ld",(long)clicks,(long)wheels];
}
@end
@interface FixtureDelegate : NSObject <NSApplicationDelegate, NSTextFieldDelegate, NSWindowDelegate>
@property(strong) NSWindow *window;
@property BOOL borderless;
@property NSRect previousFrame;
@property NSUInteger previousStyle;
@property(strong) NSTextField *input;
@property(strong) id foregroundObserver;
@end
@implementation FixtureDelegate
- (void)applicationDidFinishLaunching:(NSNotification *)note {
    NSRect area=NSScreen.screens.firstObject.visibleFrame;
    self.window=[[NSWindow alloc] initWithContentRect:area styleMask:NSWindowStyleMaskTitled|NSWindowStyleMaskClosable|NSWindowStyleMaskResizable|NSWindowStyleMaskMiniaturizable backing:NSBackingStoreBuffered defer:NO];
    self.window.title=@"Sidelet · underlying interaction fixture";
    fixtureWindow=self.window;
    self.window.collectionBehavior=NSWindowCollectionBehaviorFullScreenPrimary;
    [self.window setFrame:area display:YES];
    TestSurface *surface=[[TestSurface alloc] initWithFrame:NSMakeRect(0,0,area.size.width,area.size.height-28)];
    self.window.contentView=surface;
    status=[NSTextField labelWithString:@"Underlying clicks: 0 · wheel events: 0"];
    status.frame=NSMakeRect(20,20,900,30); [surface addSubview:status];
    NSTextField *input=[[NSTextField alloc] initWithFrame:NSMakeRect(20,65,680,42)];
    input.placeholderString=@"Disposable focus test input"; input.delegate=self; [surface addSubview:input];
    self.input=input;
    fixtureInput=input;
    NSArray *labels=@[@"Native full screen",@"Borderless full screen",@"Inspect Sidelet windows"];
    SEL actions[]={@selector(nativeFullscreen:),@selector(borderlessFullscreen:),@selector(inspect:)};
    for (int i=0;i<3;i++) {
        NSButton *button=[NSButton buttonWithTitle:labels[i] target:self action:actions[i]];
        button.frame=NSMakeRect(725+i*230,70,220,32); [surface addSubview:button];
    }
    NSButton *activate=[NSButton buttonWithTitle:@"Activate test app" target:self action:@selector(activate:)];
    activate.frame=NSMakeRect(20,110,200,25); [surface addSubview:activate];
    NSButton *keyboard=[NSButton buttonWithTitle:@"Test-only Sidelet keyboard entry" target:self action:@selector(testKeyboard:)];
    keyboard.frame=NSMakeRect(240,110,350,25); [surface addSubview:keyboard];
    keyboard.hidden=manualSession;
    if (manualSession) {
        NSTextField *instructions=[NSTextField labelWithString:@"实机验收：输入 before-sidelet → 实按 Ctrl+Option+T → ↓ / Enter / E / Esc / Esc → 不点击输入框，输入 after-escape。\n随后 Hover 标签并输入 after-hover；测试任务间隙及左侧透明区的点击与滚轮。"];
        instructions.frame=NSMakeRect(20,140,NSWidth(surface.bounds)-40,45);
        [surface addSubview:instructions];
    }
    FixtureScroll *scroll=[[FixtureScroll alloc] initWithFrame:NSMakeRect(0,140,NSWidth(surface.bounds),NSHeight(surface.bounds)-140)];
    scroll.autoresizingMask=NSViewWidthSizable|NSViewHeightSizable;
    if (manualSession) scroll.frame=NSMakeRect(0,190,NSWidth(surface.bounds),NSHeight(surface.bounds)-190);
    scroll.hasVerticalScroller=YES;
    scroll.documentView=[[ScrollDocument alloc] initWithFrame:NSMakeRect(0,0,NSWidth(surface.bounds)-20,5000)];
    scroll.documentView.autoresizingMask=NSViewWidthSizable;
    [scroll setAccessibilityLabel:@"Disposable underlying scroll area"]; [surface addSubview:scroll];
    self.window.delegate=self;
    [self.window makeKeyAndOrderFront:nil]; [NSApp activateIgnoringOtherApps:YES]; [self.window makeFirstResponder:input];
    fprintf(fixtureLog,"READY pid=%d sideletPID=%d\n",getpid(),sideletPID); fflush(fixtureLog);
    NSMutableArray *screens=[NSMutableArray new];
    for (NSScreen *screen in NSScreen.screens) [screens addObject:@{@"displayID":screen.deviceDescription[@"NSScreenNumber"],@"frame":NSStringFromRect(screen.frame),@"backingScale":@(screen.backingScaleFactor)}];
    evidence(@"environment",@{@"manualSession":@(manualSession),@"screens":screens,@"os":NSProcessInfo.processInfo.operatingSystemVersionString});
    fixtureEventMonitor=[NSEvent addLocalMonitorForEventsMatchingMask:NSEventMaskLeftMouseDown|NSEventMaskRightMouseDown|NSEventMaskOtherMouseDown handler:^NSEvent *(NSEvent *event) {
        evidence(@"mouse-down",eventFields(event)); return event;
    }];
    self.foregroundObserver=[NSWorkspace.sharedWorkspace.notificationCenter addObserverForName:NSWorkspaceDidActivateApplicationNotification object:nil queue:NSOperationQueue.mainQueue usingBlock:^(NSNotification *event) {
        [self recordFocus:@"workspace-activation"];
    }];
    [self recordFocus:@"ready"];
}
- (void)recordFocus:(NSString *)reason {
    NSMutableDictionary *record=focusFields(); record[@"reason"]=reason;
    evidence(@"focus",record);
}
- (void)testKeyboard:(id)sender {
    [self.window makeFirstResponder:self.input];
    [self recordFocus:@"before-test-entry"];
    evidence(@"test-entry",@{@"source":@"fixture-request",@"targetPID":@(sideletPID),@"fixturePID":@(getpid()),@"globalKeyTested":@NO});
    [NSDistributedNotificationCenter.defaultCenter postNotificationName:@"io.sidelet.spike.test-keyboard" object:[NSString stringWithFormat:@"%d",sideletPID] userInfo:@{@"fixturePID":@(getpid())} deliverImmediately:YES];
}
- (void)nativeFullscreen:(id)sender { [self.window toggleFullScreen:sender]; }
- (void)activate:(id)sender { evidence(@"manual-activate",focusFields()); [NSApp activateIgnoringOtherApps:YES]; [self.window makeKeyAndOrderFront:nil]; }
- (void)borderlessFullscreen:(id)sender {
    self.borderless=!self.borderless;
    if (self.borderless) {
        self.previousFrame=self.window.frame; self.previousStyle=self.window.styleMask;
        self.window.styleMask=NSWindowStyleMaskBorderless;
        [self.window setFrame:self.window.screen.frame display:YES];
    } else {
        self.window.styleMask=self.previousStyle; [self.window setFrame:self.previousFrame display:YES];
    }
    fprintf(fixtureLog,"borderless=%d\n",self.borderless); fflush(fixtureLog);
}
- (void)inspect:(id)sender {
    [self recordFocus:@"inspect"];
    NSRunningApplication *front=NSWorkspace.sharedWorkspace.frontmostApplication;
    fprintf(fixtureLog,"inspect foregroundPID=%d appActive=%d nativeFullscreen=%d frame=%s screen=%s\n",front.processIdentifier,NSApp.active,(self.window.styleMask&NSWindowStyleMaskFullScreen)!=0,NSStringFromRect(self.window.frame).UTF8String,NSStringFromRect(self.window.screen.frame).UTF8String);
    NSArray *windows=CFBridgingRelease(CGWindowListCopyWindowInfo(kCGWindowListOptionOnScreenOnly|kCGWindowListExcludeDesktopElements,kCGNullWindowID));
    NSMutableArray *bounds=[NSMutableArray new];
    for (NSDictionary *window in windows) if ([window[(id)kCGWindowOwnerPID] intValue]==sideletPID)
        [bounds addObject:window[(id)kCGWindowBounds]];
    NSData *data=[NSJSONSerialization dataWithJSONObject:bounds options:0 error:nil];
    NSString *json=[[NSString alloc] initWithData:data encoding:NSUTF8StringEncoding];
    fprintf(fixtureLog,"Sidelet onscreen=%s\n",json.UTF8String); fflush(fixtureLog);
    status.stringValue=[NSString stringWithFormat:@"Sidelet on-screen windows: %lu · front PID: %d · fixture active: %d",(unsigned long)bounds.count,front.processIdentifier,NSApp.active];
}
- (void)windowDidBecomeKey:(NSNotification *)note { fprintf(fixtureLog,"fixture key=true\n"); fflush(fixtureLog); [self recordFocus:@"window-key"]; }
- (void)windowDidResignKey:(NSNotification *)note { fprintf(fixtureLog,"fixture key=false\n"); fflush(fixtureLog); [self recordFocus:@"window-resign-key"]; }
- (void)windowDidEnterFullScreen:(NSNotification *)note { fprintf(fixtureLog,"nativeFullscreen=true\n"); fflush(fixtureLog); }
- (void)windowDidExitFullScreen:(NSNotification *)note { fprintf(fixtureLog,"nativeFullscreen=false\n"); fflush(fixtureLog); }
- (void)controlTextDidChange:(NSNotification *)note { fprintf(fixtureLog,"input=%s\n",[(NSTextField *)note.object stringValue].UTF8String); fflush(fixtureLog); [self recordFocus:@"input-change"]; }
- (void)applicationWillTerminate:(NSNotification *)note { if (self.foregroundObserver) [NSWorkspace.sharedWorkspace.notificationCenter removeObserver:self.foregroundObserver]; if (fixtureEventMonitor) [NSEvent removeMonitor:fixtureEventMonitor]; }
- (BOOL)applicationShouldTerminateAfterLastWindowClosed:(NSApplication *)app { return YES; }
@end
int main(int argc,const char **argv) {
    if ((argc!=3 && argc!=4) || (sideletPID=atoi(argv[2]))<=0) return 2;
    if (argc==4) { if (strcmp(argv[3],"manual")) return 2; manualSession=YES; }
    fixtureLog=fopen(argv[1],"w"); if (!fixtureLog) return 1;
    @autoreleasepool {
        NSApplication *app=NSApplication.sharedApplication;
        [app setActivationPolicy:NSApplicationActivationPolicyRegular];
        FixtureDelegate *delegate=[FixtureDelegate new]; app.delegate=delegate; [app run];
    }
    fclose(fixtureLog); return 0;
}
