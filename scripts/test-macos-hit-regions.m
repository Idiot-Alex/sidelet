// Read-only WindowServer probes: no synthetic input or global event injection.
#import "../internal/platform/window_darwin.m"

static int fullscreenRequests;
void sideletNativeEvent(uint64_t token, int kind, double x, double y, bool inside) { if (kind==5) fullscreenRequests++; }
static int failures;
static NSWindow *underlying;
static NSPanel *render;
static NSRect initialFrame;

@interface RoutingView : NSView
@end
@implementation RoutingView
- (BOOL)isFlipped { return YES; }
- (void)drawRect:(NSRect)rect { [NSColor.whiteColor setFill]; NSRectFill(rect); }
@end
@interface RoutingPanel : NSPanel
@property(strong) NSView *webView;
@end
@implementation RoutingPanel
@end

static void check(BOOL passed, NSString *name) {
    printf("%s %s\n",passed ? "PASS" : "FAIL",name.UTF8String);
    if (!passed) failures++;
}
static NSInteger route(NSPoint point) { return [NSWindow windowNumberAtPoint:point belowWindowWithWindowNumber:0]; }
static NSPoint screenPoint(double x, double y) { return [render convertPointToScreen:NSMakePoint(x,NSHeight(render.frame)-y)]; }
static void settled(dispatch_block_t block) {
    dispatch_after(dispatch_time(DISPATCH_TIME_NOW,150*NSEC_PER_MSEC),dispatch_get_main_queue(),^{ @autoreleasepool { block(); } });
}
static void finish(void) {
    SLStopWatching();
    SLClose((__bridge void *)render); [render close]; [underlying close];
    printf("Native routing assertions: %d failures\n",failures);
    exit(failures ? 1 : 0);
}

@interface RoutingDelegate : NSObject <NSApplicationDelegate>
@end
@implementation RoutingDelegate
- (void)applicationDidFinishLaunching:(NSNotification *)note {
    @autoreleasepool {
    NSRect area=NSScreen.screens.firstObject.visibleFrame;
    NSRect base=NSMakeRect(NSMidX(area)-250,NSMidY(area)-200,500,400);
    underlying=[[NSWindow alloc] initWithContentRect:base styleMask:NSWindowStyleMaskBorderless backing:NSBackingStoreBuffered defer:NO];
    underlying.releasedWhenClosed=NO;
    underlying.backgroundColor=NSColor.blueColor; underlying.opaque=YES;
    [underlying orderFrontRegardless];
    initialFrame=NSMakeRect(base.origin.x+50,base.origin.y+50,300,240);
    RoutingPanel *panel=[[RoutingPanel alloc] initWithContentRect:initialFrame styleMask:NSWindowStyleMaskBorderless | NSWindowStyleMaskNonactivatingPanel backing:NSBackingStoreBuffered defer:NO];
    panel.releasedWhenClosed=NO;
    panel.webView=[[RoutingView alloc] initWithFrame:NSMakeRect(0,0,300,240)];
    panel.contentView=panel.webView; render=panel;
    SLEnableMemoryDiagnostics();
    check(SLBind((__bridge void *)render,1),@"bind nonactivating render panel");
    static const SLRect collapsed[]={{280,12,20,44},{280,62,20,44}};
    SLRegions((__bridge void *)render,collapsed,2,300,240);
    SLShow((__bridge void *)render);
    settled(^{
        SLWindowState *state=stateFor(render);
        check(state.inputPanels.count==2,@"two input rectangles, shared render view");
        @autoreleasepool {
            check(memoryPanels.allObjects.count==2 && memoryInputViews.allObjects.count==2 && memoryPanelsCreated==2,@"weak memory registries observe two live helpers without new allocations");
        }
        check(route(screenPoint(290,30))==state.inputPanels[0].windowNumber,@"first collapsed tag routes BEFORE mouse entry");
        check(route(screenPoint(290,80))==state.inputPanels[1].windowNumber,@"second tag routes independently");
        check(route(screenPoint(290,59))==underlying.windowNumber,@"six-point gap reaches underlying window");
        check(route(screenPoint(150,30))==underlying.windowNumber,@"transparent render area reaches underlying window");
        check(!render.isKeyWindow && !state.inputPanels[0].isKeyWindow,@"passive render and helpers do not take keyboard focus");
        check(![state.inputPanels[0] canBecomeKeyWindow],@"helper refuses key window status");
        // Inspect Carbon event DATA only; never post a key to the system.
        EventRef unrelated=NULL;
        CreateEvent(NULL,kEventClassKeyboard,kEventHotKeyPressed,GetCurrentEventTime(),0,&unrelated);
        EventHotKeyID unrelatedID={'othr',1};
        SetEventParameter(unrelated,kEventParamDirectObject,typeEventHotKeyID,sizeof(unrelatedID),&unrelatedID);
        check(keyboardHandlerProc(NULL,unrelated,NULL)==eventNotHandledErr,@"unrelated Carbon hotkeys remain available to other handlers");
        ReleaseEvent(unrelated);
        // Construct event DATA only. No event is sent or posted in these tests.
        NSWindow *source=state.inputPanels[0];
        NSEvent *original=[NSEvent mouseEventWithType:NSEventTypeLeftMouseDown location:NSMakePoint(5,20) modifierFlags:NSEventModifierFlagShift timestamp:10 windowNumber:source.windowNumber context:nil eventNumber:7 clickCount:2 pressure:0.5];
        NSEvent *converted=retargetMouseEvent(original,source,render);
        check(converted.window==render && converted.windowNumber==render.windowNumber,@"forwarded event data identifies shared render window");
        NSPoint expected=[render convertPointFromScreen:[source convertPointToScreen:original.locationInWindow]];
        check(NSEqualPoints(converted.locationInWindow,expected),@"forwarded event preserves exact screen position");
        check(converted.clickCount==2 && converted.type==NSEventTypeLeftMouseDown,@"double-click count and mouse type survive forwarding");
        check(converted.modifierFlags==original.modifierFlags && converted.timestamp==10 && converted.pressure==original.pressure,@"event modifiers, timestamp and pressure survive forwarding");
        SLRect expanded[]={{100,12,200,44},{280,62,20,44}};
        SLRegions((__bridge void *)render,expanded,2,300,240);
        settled(^{
            check(route(screenPoint(150,30))==state.inputPanels[0].windowNumber,@"expansion installs the larger hit rectangle");
            SLRegions((__bridge void *)render,collapsed,2,300,240);
            settled(^{
                check(route(screenPoint(150,30))==underlying.windowNumber,@"collapse removes the old expanded rectangle");
                SLRect moved={initialFrame.origin.x+50,primaryTop()-NSMaxY(initialFrame),300,240};
                check(SLMove((__bridge void *)render,moved),@"move render panel");
                settled(^{
                    check(route(screenPoint(290,30))==state.inputPanels[0].windowNumber,@"input panels follow render movement");
                    SLHide((__bridge void *)render);
                    settled(^{
                        check(route(screenPoint(290,30))==underlying.windowNumber,@"hidden helpers cannot intercept input");
                        SLRect full={0,0,300,240};
                        SLRegions((__bridge void *)render,&full,1,300,240); SLShow((__bridge void *)render);
                        settled(^{
                            check(state.directInput && state.inputPanels.count==0,@"full Quick Card needs no helper panels");
                            @autoreleasepool {
                                printf("Memory registry after removal: panels=%lu views=%lu children=%lu\n",(unsigned long)memoryPanels.allObjects.count,(unsigned long)memoryInputViews.allObjects.count,(unsigned long)render.childWindows.count);
                                // AppKit may hold closed windows until a later
                                // autorelease-pool drain. This is not a leak test.
                                check(state.inputPanels.count==0 && render.childWindows.count==0,@"full-viewport switch removes helper ownership and parent links");
                            }
                            check(route(screenPoint(150,100))==render.windowNumber,@"full Quick Card receives directly before hover");
                            CGRect screen=CGRectMake(0,0,1728,1117);
                            NSEdgeInsets safe=NSEdgeInsetsMake(32,0,0,0);
                            check(coversFullscreenDisplay(CGRectMake(0,33,1728,1084),screen,safe),@"observed notched Mac native fullscreen is recognized");
                            check(!coversFullscreenDisplay(CGRectMake(0,33,1728,1007),screen,safe),@"maximized work-area window is not fullscreen");
                            CGRect external=CGRectMake(-1920,0,1920,1080);
                            check(coversFullscreenDisplay(external,external,NSEdgeInsetsMake(0,0,0,0)),@"unnotched negative-origin display recognizes fullscreen");
                            check(!coversFullscreenDisplay(CGRectMake(-1920,0,1600,1080),external,NSEdgeInsetsMake(0,0,0,0)),@"partial-width window is not fullscreen");
                            // Isolate timer semantics from unrelated physical input.
                            if (globalMouseMonitor) [NSEvent removeMonitor:globalMouseMonitor];
                            if (localMouseMonitor) [NSEvent removeMonitor:localMouseMonitor];
                            globalMouseMonitor=localMouseMonitor=nil;
                            foregroundWatchToken=9876;
                            requestFullscreenCheck(0.03); requestFullscreenCheck(0.05);
                            settled(^{
                                check(fullscreenRequests==1,@"fullscreen event burst produces one delayed check");
                                settled(^{
                                    check(fullscreenRequests==1,@"no input causes no recurring fullscreen checks");
                                    requestFullscreenCheck(0.05); SLStopWatching();
                                    settled(^{
                                        check(fullscreenRequests==1,@"watch cleanup cancels pending fullscreen check");
                                        finish();
                                    });
                                });
                            });
                        });
                    });
                });
            });
        });
    });
    }
}
@end
int main(void) {
    @autoreleasepool {
        NSApplication *app=NSApplication.sharedApplication;
        [app setActivationPolicy:NSApplicationActivationPolicyAccessory];
        RoutingDelegate *delegate=[RoutingDelegate new]; app.delegate=delegate;
        [app run];
    }
    return 0;
}
