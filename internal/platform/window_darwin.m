//go:build darwin

#import <Cocoa/Cocoa.h>
#import <ApplicationServices/ApplicationServices.h>
#import <Carbon/Carbon.h>
#import <objc/runtime.h>
#include "window_darwin.h"

extern void sideletNativeEvent(uint64_t id, int kind, double x, double y, bool inside);

@interface SLWindowState : NSObject
@property(nonatomic, weak) NSWindow *window;
@property(nonatomic) uint64_t token;
@property(nonatomic) BOOL active;
@property(nonatomic) BOOL activationPending;
@property(nonatomic) NSInteger hit;
@property(nonatomic) double viewportWidth;
@property(nonatomic) double viewportHeight;
@property(nonatomic, strong) NSArray<NSValue *> *regions;
@property(nonatomic, strong) NSMutableArray *observers;
@property(nonatomic, weak) NSView *webview;
@property(nonatomic) BOOL directInput;
@property(nonatomic) BOOL quickAdd;
@property(nonatomic) NSUInteger forwardedEvents;
@property(nonatomic) NSPoint pointerScreen;
@property(nonatomic, strong) NSMutableArray<NSPanel *> *inputPanels;
@end
@implementation SLWindowState
@end

static const char SLStateKey;
static NSMutableDictionary<NSNumber *, SLWindowState *> *states;
static id globalMouseMonitor, localMouseMonitor;
static NSMutableArray *workspaceObservers;
static uint64_t foregroundWatchToken;
static dispatch_block_t fullscreenCheck;
static NSTimeInterval fullscreenCheckDue;
static NSString *lastFailure;
static IMP originalCanBecomeKey, originalFirstMouse;
static EventHotKeyRef keyboardShortcut;
static EventHandlerRef keyboardHandler;
static uint64_t keyboardToken;
static EventHotKeyRef quickAddShortcut;
static EventHandlerRef quickAddHandler;
static uint64_t quickAddToken;
static id interactionTestObserver;
static uint64_t interactionTestToken;
static SLWindowState *stateFor(NSWindow *window);

static OSStatus keyboardHandlerProc(EventHandlerCallRef next, EventRef event, void *context) {
    EventHotKeyID id;
    OSStatus result = GetEventParameter(event,kEventParamDirectObject,typeEventHotKeyID,NULL,sizeof(id),NULL,&id);
    if (result!=noErr || id.signature!='SLet' || id.id!=1 || !keyboardToken) return eventNotHandledErr;
    sideletNativeEvent(keyboardToken,7,0,0,false);
    return noErr;
}
static void stopKeyboardShortcut(void) {
    if (keyboardShortcut) UnregisterEventHotKey(keyboardShortcut);
    if (keyboardHandler) RemoveEventHandler(keyboardHandler);
    keyboardShortcut=NULL; keyboardHandler=NULL; keyboardToken=0;
}
static OSStatus quickAddHandlerProc(EventHandlerCallRef next, EventRef event, void *context) {
    EventHotKeyID id;
    OSStatus result=GetEventParameter(event,kEventParamDirectObject,typeEventHotKeyID,NULL,sizeof(id),NULL,&id);
    if(result!=noErr || id.signature!='SLet' || id.id!=2 || !quickAddToken) return eventNotHandledErr;
    sideletNativeEvent(quickAddToken,8,0,0,false);return noErr;
}
static void stopQuickAddShortcut(void) {
    if(quickAddShortcut) UnregisterEventHotKey(quickAddShortcut);
    if(quickAddHandler) RemoveEventHandler(quickAddHandler);
    quickAddShortcut=NULL;quickAddHandler=NULL;quickAddToken=0;
}
void SLConfigureQuickAdd(void *pointer) {
    NSWindow *window=(__bridge NSWindow *)pointer;
    stateFor(window).quickAdd=YES;window.title=@"Sidelet · 快速添加";
}
void SLConfigureQuickCard(void *pointer) {
    NSWindow *window=(__bridge NSWindow *)pointer;
    stateFor(window).quickAdd=NO;window.title=@"Sidelet · 快速操作";
}
bool SLRegisterQuickAddShortcut(void *pointer) {
    SLWindowState *state=stateFor((__bridge NSWindow *)pointer);
    if(!state){lastFailure=@"quick add requires a bound panel";return false;}
    if(quickAddShortcut){if(quickAddToken==state.token)return true;lastFailure=@"quick add shortcut has another owner";return false;}
    EventTypeSpec type={kEventClassKeyboard,kEventHotKeyPressed};
    OSStatus result=InstallApplicationEventHandler(quickAddHandlerProc,1,&type,NULL,&quickAddHandler);
    if(result==noErr){EventHotKeyID id={'SLet',2};result=RegisterEventHotKey(kVK_Space,controlKey|shiftKey,id,GetApplicationEventTarget(),kEventHotKeyExclusive,&quickAddShortcut);}
    if(result!=noErr){lastFailure=[NSString stringWithFormat:@"RegisterEventHotKey OSStatus=%d",(int)result];stopQuickAddShortcut();return false;}
    quickAddToken=state.token;return true;
}
static void stopInteractionTest(void) {
    if (interactionTestObserver) [NSDistributedNotificationCenter.defaultCenter removeObserver:interactionTestObserver];
    interactionTestObserver=nil; interactionTestToken=0;
}
// Weak registries observe leaked/detached objects without retaining them.
// Created only for an explicitly instrumented run, on the AppKit thread.
static NSHashTable *memoryStates, *memoryPanels, *memoryInputViews;
static NSUInteger memoryPanelsCreated, memoryInputViewsCreated;
void SLEnableMemoryDiagnostics(void) {
    if (memoryStates) return;
    memoryStates = [NSHashTable weakObjectsHashTable];
    memoryPanels = [NSHashTable weakObjectsHashTable];
    memoryInputViews = [NSHashTable weakObjectsHashTable];
    for (SLWindowState *state in states.allValues) {
        [memoryStates addObject:state];
        for (NSPanel *panel in state.inputPanels) {
            [memoryPanels addObject:panel]; [memoryInputViews addObject:panel.contentView];
        }
    }
}
static void updatePointerAt(SLWindowState *state, NSPoint screenPoint);
static void updatePointer(SLWindowState *state);
static void syncInputPanels(SLWindowState *state);
static void requestFullscreenCheck(NSTimeInterval delay) {
    if (!foregroundWatchToken) return;
    NSTimeInterval due = NSProcessInfo.processInfo.systemUptime + delay;
    // Keep the longer Space-animation settling delay if pointer events arrive
    // in the meantime. Otherwise coalesce the current burst of external input.
    if (fullscreenCheck && due < fullscreenCheckDue) return;
    if (fullscreenCheck) dispatch_block_cancel(fullscreenCheck);
    fullscreenCheckDue = due;
    uint64_t token = foregroundWatchToken;
    fullscreenCheck = dispatch_block_create(0, ^{
        if (token != foregroundWatchToken) return;
        fullscreenCheck = nil;
        SLWindowState *state = states.allValues.firstObject;
        if (state) sideletNativeEvent(state.token, 5, 0, 0, false);
    });
    dispatch_after(dispatch_time(DISPATCH_TIME_NOW,(int64_t)(delay*NSEC_PER_SEC)),dispatch_get_main_queue(),fullscreenCheck);
}
static NSEvent *retargetMouseEvent(NSEvent *event, NSWindow *source, NSWindow *target) {
    NSPoint screen = [source convertPointToScreen:event.locationInWindow];
    return [NSEvent mouseEventWithType:event.type location:[target convertPointFromScreen:screen] modifierFlags:event.modifierFlags timestamp:event.timestamp windowNumber:target.windowNumber context:nil eventNumber:event.eventNumber clickCount:event.clickCount pressure:event.pressure];
}

// The render window stays mouse-transparent. Small nonactivating child panels
// BELOW it own the hit rectangles before any mouse event arrives. They share
// the existing WebView and forward only events already delivered to this app.
@interface SLInputPanel : NSPanel
@end
@implementation SLInputPanel
- (BOOL)canBecomeKeyWindow { return NO; }
- (BOOL)canBecomeMainWindow { return NO; }
- (BOOL)isAccessibilityElement { return NO; }
@end

@interface SLInputView : NSView
@property(nonatomic, weak) SLWindowState *state;
@property(nonatomic, strong) NSTrackingArea *tracking;
@end
@implementation SLInputView
- (BOOL)acceptsFirstMouse:(NSEvent *)event { return YES; }
- (void)updateTrackingAreas {
    [super updateTrackingAreas];
    if (self.tracking) [self removeTrackingArea:self.tracking];
    self.tracking = [[NSTrackingArea alloc] initWithRect:NSZeroRect options:NSTrackingMouseEnteredAndExited | NSTrackingMouseMoved | NSTrackingActiveAlways | NSTrackingInVisibleRect owner:self userInfo:nil];
    [self addTrackingArea:self.tracking];
}
- (void)drawRect:(NSRect)rect {
    // Opaque backing makes the helper eligible for WindowServer hit testing;
    // the actual label and its text are painted by the render window above it.
    [[NSColor colorWithDeviceRed:233.0/255 green:236.0/255 blue:229.0/255 alpha:1] setFill];
    NSRectFill(rect);
}
- (void)pointer:(NSEvent *)event {
    if (!self.window) return;
    updatePointerAt(self.state, [self.window convertPointToScreen:event.locationInWindow]);
}
- (void)forwardMouse:(NSEvent *)event {
    SLWindowState *state = self.state;
    NSWindow *target = state.window;
    if (!target || !target.visible) return;
    [self pointer:event];
    NSEvent *converted = retargetMouseEvent(event,self.window,target);
    state.forwardedEvents++;
    [target sendEvent:converted];
}
- (void)mouseEntered:(NSEvent *)event { updatePointer(self.state); }
- (void)mouseExited:(NSEvent *)event { updatePointer(self.state); }
- (void)mouseMoved:(NSEvent *)event { [self forwardMouse:event]; }
- (void)mouseDown:(NSEvent *)event { [self forwardMouse:event]; }
- (void)mouseUp:(NSEvent *)event { [self forwardMouse:event]; }
- (void)mouseDragged:(NSEvent *)event { [self forwardMouse:event]; }
- (void)rightMouseDown:(NSEvent *)event { [self forwardMouse:event]; }
- (void)rightMouseUp:(NSEvent *)event { [self forwardMouse:event]; }
- (void)rightMouseDragged:(NSEvent *)event { [self forwardMouse:event]; }
- (void)otherMouseDown:(NSEvent *)event { [self forwardMouse:event]; }
- (void)otherMouseUp:(NSEvent *)event { [self forwardMouse:event]; }
- (void)otherMouseDragged:(NSEvent *)event { [self forwardMouse:event]; }
// Stack labels do not scroll. Quick Card uses its original rectangular window
// directly, so WebKit receives unmodified native wheel events there.
- (void)scrollWheel:(NSEvent *)event { [self pointer:event]; }
@end

static double primaryTop(void) { return NSMaxY([NSScreen screens].firstObject.frame); }
static SLWindowState *stateFor(NSWindow *window) { return objc_getAssociatedObject(window, &SLStateKey); }
bool SLRegisterKeyboardShortcut(void *pointer) {
    SLWindowState *state = stateFor((__bridge NSWindow *)pointer);
    if (!state) { lastFailure=@"shortcut requires a bound panel"; return false; }
    if (keyboardShortcut) {
        if (keyboardToken==state.token) return true;
        lastFailure=@"shortcut is already owned by another panel"; return false;
    }
    EventTypeSpec type = {kEventClassKeyboard,kEventHotKeyPressed};
    OSStatus result = InstallApplicationEventHandler(keyboardHandlerProc,1,&type,NULL,&keyboardHandler);
    if (result==noErr) {
        EventHotKeyID id = {'SLet',1};
        // Wails beta.27 uses non-exclusive registration. Another process's
        // exclusive binding can suppress it without returning a failure.
        // Own the combination exclusively so conflicts become visible errors.
        result = RegisterEventHotKey(kVK_ANSI_T,controlKey|optionKey,id,GetApplicationEventTarget(),kEventHotKeyExclusive,&keyboardShortcut);
    }
    if (result!=noErr) {
        lastFailure=[NSString stringWithFormat:@"RegisterEventHotKey OSStatus=%d%@",(int)result,result==eventHotKeyExistsErr?@" (shortcut already in use)":@""];
        stopKeyboardShortcut(); return false;
    }
    keyboardToken=state.token;
    return true;
}
bool SLEnableInteractionTest(void *pointer) {
    SLWindowState *state = stateFor((__bridge NSWindow *)pointer);
    if (!state) return false;
    stopInteractionTest(); interactionTestToken=state.token;
    uint64_t token=state.token;
    NSString *target=[NSString stringWithFormat:@"%d",getpid()];
    interactionTestObserver=[NSDistributedNotificationCenter.defaultCenter addObserverForName:@"io.sidelet.spike.test-keyboard" object:target queue:NSOperationQueue.mainQueue usingBlock:^(NSNotification *note) {
        if (interactionTestToken!=token) return;
        int pid=[note.userInfo[@"fixturePID"] intValue];
        NSRunningApplication *fixture=[NSRunningApplication runningApplicationWithProcessIdentifier:pid];
        // An opt-in test entry from the active, disposable fixture only.
        if (pid>0 && [fixture.bundleIdentifier isEqualToString:@"io.sidelet.interactionfixture"] && NSWorkspace.sharedWorkspace.frontmostApplication.processIdentifier==pid)
            sideletNativeEvent(token,[note.userInfo[@"operation"] isEqualToString:@"quick-add"]?9:6,0,0,false);
    }];
    return true;
}
static BOOL canBecomeKey(id self, SEL selector) {
    SLWindowState *state = stateFor(self);
    return state ? state.active : ((BOOL (*)(id,SEL))originalCanBecomeKey)(self,selector);
}
static BOOL firstMouse(id self, SEL selector, NSEvent *event) {
    return stateFor(((NSView *)self).window) ? YES : ((BOOL (*)(id,SEL,NSEvent *))originalFirstMouse)(self,selector,event);
}
// Preserve AppKit/WebKit's KVO-generated classes. Replacing an object's isa
// breaks their observer metadata during resize. These guarded overrides affect
// only bound overlay panels/views; unbound views use the original implementation.
static void installPanelBehavior(NSWindow *window, NSView *webview) {
    if (!originalCanBecomeKey) {
        Class cls = window.class;
        Method method = class_getInstanceMethod(cls, @selector(canBecomeKeyWindow));
        originalCanBecomeKey = method_getImplementation(method);
        class_replaceMethod(cls, @selector(canBecomeKeyWindow), (IMP)canBecomeKey, method_getTypeEncoding(method));
    }
    if (webview && !originalFirstMouse) {
        Class cls = webview.class;
        Method method = class_getInstanceMethod(cls, @selector(acceptsFirstMouse:));
        originalFirstMouse = method_getImplementation(method);
        if (!class_addMethod(cls, @selector(acceptsFirstMouse:), (IMP)firstMouse, method_getTypeEncoding(method)))
            class_replaceMethod(cls, @selector(acceptsFirstMouse:), (IMP)firstMouse, method_getTypeEncoding(method));
    }
}

static void updatePointerAt(SLWindowState *state, NSPoint screenPoint) {
    NSWindow *window = state.window;
    if (!window) return;
    state.pointerScreen = screenPoint;
    NSView *view = state.webview ?: window.contentView;
    NSPoint point = [view convertPoint:[window convertPointFromScreen:screenPoint] fromView:nil];
    if (!view.isFlipped) point.y = NSHeight(view.bounds) - point.y;
    double sx = state.viewportWidth > 0 ? NSWidth(view.bounds) / state.viewportWidth : 1;
    double sy = state.viewportHeight > 0 ? NSHeight(view.bounds) / state.viewportHeight : 1;
    NSInteger hit = -1;
    if (window.visible && NSPointInRect(screenPoint,window.frame) && sx > 0 && sy > 0) {
        for (NSUInteger i = 0; i < state.regions.count; i++) {
            if (NSPointInRect(point, state.regions[i].rectValue)) { hit = i; break; }
        }
    }
    if (hit != state.hit) {
        state.hit = hit;
        sideletNativeEvent(state.token, 1, sx > 0 ? point.x / sx : 0, sy > 0 ? point.y / sy : 0, hit >= 0);
    }
}
static void updatePointer(SLWindowState *state) { updatePointerAt(state, state.pointerScreen); }
static void syncInputPanels(SLWindowState *state) {
    NSWindow *window = state.window;
    if (!window) return;
    NSView *view = state.webview ?: window.contentView;
    NSMutableArray<NSValue *> *frames = [NSMutableArray new];
    if (!state.directInput) for (NSValue *value in state.regions) {
        NSRect rect = value.rectValue;
        if (!view.isFlipped) rect.origin.y = NSHeight(view.bounds)-NSMaxY(rect);
        NSRect frame = NSIntersectionRect([window convertRectToScreen:[view convertRect:rect toView:nil]],window.frame);
        if (!NSIsEmptyRect(frame)) [frames addObject:[NSValue valueWithRect:frame]];
    }
    while (state.inputPanels.count > frames.count) {
        NSPanel *panel = state.inputPanels.lastObject;
        [window removeChildWindow:panel]; [panel orderOut:nil]; [panel close];
        [state.inputPanels removeLastObject];
    }
    for (NSUInteger i=0; i<frames.count; i++) {
        SLInputPanel *panel;
        if (i >= state.inputPanels.count) {
            panel = [[SLInputPanel alloc] initWithContentRect:frames[i].rectValue styleMask:NSWindowStyleMaskBorderless | NSWindowStyleMaskNonactivatingPanel backing:NSBackingStoreBuffered defer:NO];
            panel.releasedWhenClosed = NO;
            panel.opaque = YES; panel.hasShadow = NO;
            panel.backgroundColor = [NSColor colorWithDeviceRed:233.0/255 green:236.0/255 blue:229.0/255 alpha:1];
            panel.hidesOnDeactivate = NO; panel.floatingPanel = YES;
            panel.acceptsMouseMovedEvents = YES;
            panel.excludedFromWindowsMenu = YES;
            panel.collectionBehavior = window.collectionBehavior;
            SLInputView *input = [[SLInputView alloc] initWithFrame:NSMakeRect(0,0,NSWidth(panel.frame),NSHeight(panel.frame))];
            input.state = state; panel.contentView = input;
            if (memoryStates) {
                memoryPanelsCreated++; memoryInputViewsCreated++;
                [memoryPanels addObject:panel]; [memoryInputViews addObject:input];
            }
            [state.inputPanels addObject:panel];
            [window addChildWindow:panel ordered:NSWindowBelow];
        } else panel = (SLInputPanel *)state.inputPanels[i];
        panel.level = window.level;
        [panel setFrame:frames[i].rectValue display:YES];
        if (window.visible) [panel orderWindow:NSWindowBelow relativeTo:window.windowNumber];
        else [panel orderOut:nil];
    }
    // Routing is determined by installed rectangles, never by an asynchronous
    // global mouse notification. A full-viewport Quick Card receives directly.
    window.ignoresMouseEvents = !state.directInput;
}
static void monitorMouse(NSEvent *event) {
    if (!SLForegroundIsOurs() && (event.type == NSEventTypeMouseMoved || event.type == NSEventTypeLeftMouseUp || event.type == NSEventTypeRightMouseUp || event.type == NSEventTypeOtherMouseUp))
        requestFullscreenCheck(0.15);
    NSArray<SLWindowState *> *snapshot = states.allValues;
    NSPoint screen;
    if (event.window) screen = [event.window convertPointToScreen:event.locationInWindow];
    else if (event.CGEvent) {
        CGPoint point = CGEventGetLocation(event.CGEvent);
        screen = NSMakePoint(point.x,primaryTop()-point.y);
    } else return;
    // App-scoped events can carry placeholder coordinates on both local and
    // global paths. Only actual display locations may replace the last pointer.
    BOOL onScreen = NO;
    for (NSScreen *display in NSScreen.screens) if (NSPointInRect(screen,display.frame)) { onScreen = YES; break; }
    if (!onScreen) return;
    for (SLWindowState *state in snapshot) updatePointerAt(state, screen);
    if (event.type == NSEventTypeLeftMouseDown || event.type == NSEventTypeRightMouseDown || event.type == NSEventTypeOtherMouseDown) {
        BOOL inside = NO;
        for (SLWindowState *state in snapshot) if (state.hit >= 0 && state.window.visible) inside = YES;
        if (!inside && snapshot.count) sideletNativeEvent(snapshot.firstObject.token, 3, 0, 0, false);
    }
}
static void ensureMouseMonitors(void) {
    if (globalMouseMonitor) return;
    NSEventMask mask = NSEventMaskMouseMoved | NSEventMaskLeftMouseDragged | NSEventMaskRightMouseDragged | NSEventMaskOtherMouseDragged | NSEventMaskLeftMouseDown | NSEventMaskRightMouseDown | NSEventMaskOtherMouseDown | NSEventMaskLeftMouseUp | NSEventMaskRightMouseUp | NSEventMaskOtherMouseUp | NSEventMaskScrollWheel;
    globalMouseMonitor = [NSEvent addGlobalMonitorForEventsMatchingMask:mask handler:^(NSEvent *event) { monitorMouse(event); }];
    localMouseMonitor = [NSEvent addLocalMonitorForEventsMatchingMask:mask handler:^NSEvent *(NSEvent *event) { monitorMouse(event); return event; }];
}
static BOOL focusActivePanel(SLWindowState *state) {
    NSWindow *window=state.window;
    [window makeKeyAndOrderFront:nil];
    BOOL accepted=[window makeFirstResponder:state.webview];
    state.activationPending=NO;
    if (!accepted) state.active=NO;
    return accepted;
}

bool SLBind(void *pointer, uint64_t token) {
    NSWindow *window = (__bridge NSWindow *)pointer;
    if (![window isKindOfClass:NSPanel.class] || !(window.styleMask & NSWindowStyleMaskNonactivatingPanel)) return false;
    if (!states) states = [NSMutableDictionary new];
    SLWindowState *state = [SLWindowState new];
    if (memoryStates) [memoryStates addObject:state];
    state.window = window;
    state.token = token;
    state.hit = NSIntegerMin;
    state.pointerScreen = NSEvent.mouseLocation;
    state.regions = @[];
    state.observers = [NSMutableArray new];
    state.inputPanels = [NSMutableArray new];
    // Frameless Wails panels otherwise have an empty native title, which also
    // prevents accessibility clients from matching them to WindowServer data.
    if (!window.title.length) window.title = [NSString stringWithFormat:@"Sidelet · 桌面任务 %llu",(unsigned long long)token];
    objc_setAssociatedObject(window, &SLStateKey, state, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
    NSView *webview = [window valueForKey:@"webView"];
    // NSWindow transparency does not disable WKWebView's white page backing.
    // Wails beta.27 defaults to a no-op transparency switch unless all of its
    // private Mac APIs are enabled. Scope this WebKit compatibility key to our
    // two overlay roles instead; the ordinary task/settings window stays opaque.
    if ([webview isKindOfClass:NSClassFromString(@"WKWebView")]) {
        @try {
            [webview setValue:@NO forKey:@"drawsBackground"];
            [webview setValue:NSColor.clearColor forKey:@"underPageBackgroundColor"];
        } @catch (NSException *exception) {
            lastFailure = [@"Cannot make the overlay WebView transparent: " stringByAppendingString:exception.reason ?: @"unsupported WebKit"];
            objc_setAssociatedObject(window, &SLStateKey, nil, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
            return false;
        }
    }
    state.webview = webview;
    installPanelBehavior(window, webview);
    window.acceptsMouseMovedEvents = YES;
    window.movable = NO;
    window.movableByWindowBackground = NO;
    window.hasShadow = NO;
    window.opaque = NO;
    window.backgroundColor = NSColor.clearColor;
    ((NSPanel *)window).hidesOnDeactivate = NO;
    states[@(token)] = state;
    NSNotificationCenter *center = NSNotificationCenter.defaultCenter;
    __weak SLWindowState *weakState=state;
    [state.observers addObject:[center addObserverForName:NSApplicationDidBecomeActiveNotification object:NSApp queue:nil usingBlock:^(NSNotification *note) {
        SLWindowState *pending=weakState;
        if (pending.active && pending.activationPending && !focusActivePanel(pending))
            sideletNativeEvent(token,3,0,0,false);
    }]];
    [state.observers addObject:[center addObserverForName:NSWindowDidResignKeyNotification object:window queue:nil usingBlock:^(NSNotification *note) { sideletNativeEvent(token, 3, 0, 0, false); }]];
    [state.observers addObject:[center addObserverForName:NSWindowDidChangeScreenNotification object:window queue:nil usingBlock:^(NSNotification *note) { sideletNativeEvent(token, 2, 0, 0, false); }]];
    [state.observers addObject:[center addObserverForName:NSApplicationDidChangeScreenParametersNotification object:nil queue:nil usingBlock:^(NSNotification *note) { sideletNativeEvent(token, 2, 0, 0, false); }]];
    ensureMouseMonitors();
    syncInputPanels(state);
    SLPassive(pointer);
    return true;
}
// Only the opaque task/settings window receives a background; overlays stay clear.
void SLControlTheme(void *pointer, int theme) {
    if (!pointer) return;
    NSWindow *window=(__bridge NSWindow *)pointer;
    window.appearance=[NSAppearance appearanceNamed:theme==2 ? NSAppearanceNameDarkAqua : NSAppearanceNameAqua];
    window.titlebarAppearsTransparent=YES;
    if (theme==2) window.backgroundColor=[NSColor colorWithSRGBRed:28/255.0 green:32/255.0 blue:31/255.0 alpha:1];
    else if (theme==1) window.backgroundColor=[NSColor colorWithSRGBRed:244/255.0 green:239/255.0 blue:229/255.0 alpha:1];
    else window.backgroundColor=[NSColor colorWithSRGBRed:244/255.0 green:246/255.0 blue:244/255.0 alpha:1];
}

void SLClose(void *pointer) {
    NSWindow *window = (__bridge NSWindow *)pointer;
    SLWindowState *state = stateFor(window);
    if (!state) return;
    if (keyboardToken==state.token) stopKeyboardShortcut();
    if (quickAddToken==state.token) stopQuickAddShortcut();
    if (interactionTestToken==state.token) stopInteractionTest();
    for (NSPanel *panel in state.inputPanels) { [window removeChildWindow:panel]; [panel orderOut:nil]; [panel close]; }
    [state.inputPanels removeAllObjects];
    for (id observer in state.observers) [NSNotificationCenter.defaultCenter removeObserver:observer];
    [states removeObjectForKey:@(state.token)];
    objc_setAssociatedObject(window, &SLStateKey, nil, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
    if (!states.count) {
        if (globalMouseMonitor) [NSEvent removeMonitor:globalMouseMonitor];
        if (localMouseMonitor) [NSEvent removeMonitor:localMouseMonitor];
        globalMouseMonitor = localMouseMonitor = nil;
    }
}
void SLPassive(void *pointer) {
    NSWindow *window = (__bridge NSWindow *)pointer;
    stateFor(window).active = NO;
    stateFor(window).activationPending = NO;
    if (window.isKeyWindow) [window resignKeyWindow];
    window.level = NSFloatingWindowLevel;
}
bool SLActivate(void *pointer) {
    NSWindow *window = (__bridge NSWindow *)pointer;
    SLWindowState *state = stateFor(window);
    if (!state || !state.webview) return false;
    state.active = YES;
    state.activationPending = !NSApp.active;
    [NSApp activateIgnoringOtherApps:YES];
    // Activation from a different app completes on a later AppKit turn.
    // Focus the panel on DidBecomeActive; do not let the earlier workspace
    // notification mistake this pending transition for loss of focus.
    if (state.activationPending) { [window orderFrontRegardless]; return true; }
    return focusActivePanel(state);
}
void SLShow(void *pointer) { NSWindow *window = (__bridge NSWindow *)pointer; [window orderFrontRegardless]; syncInputPanels(stateFor(window)); updatePointer(stateFor(window)); }
void SLHide(void *pointer) { NSWindow *window = (__bridge NSWindow *)pointer; for (NSPanel *panel in stateFor(window).inputPanels) [panel orderOut:nil]; [window orderOut:nil]; updatePointer(stateFor(window)); }
const char *SLLastError(void) { return (lastFailure ?: @"unknown AppKit error").UTF8String; }
bool SLMove(void *pointer, SLRect rect) {
    NSWindow *window = (__bridge NSWindow *)pointer;
    @try {
        [window setFrame:NSMakeRect(rect.x, primaryTop()-rect.y-rect.height, rect.width, rect.height) display:YES];
        syncInputPanels(stateFor(window));
        updatePointer(stateFor(window));
        return true;
    } @catch (NSException *exception) {
        lastFailure = [NSString stringWithFormat:@"%@: %@",exception.name,exception.reason];
        fprintf(stderr,"Sidelet Move exception: %s\n",lastFailure.UTF8String);
        return false;
    }
}
void SLRegions(void *pointer, const SLRect *rects, int count, double width, double height) {
    NSWindow *window = (__bridge NSWindow *)pointer;
    SLWindowState *state = stateFor(window);
    state.viewportWidth = width; state.viewportHeight = height;
    NSView *view = state.webview ?: window.contentView;
    double sx = NSWidth(view.bounds)/width, sy = NSHeight(view.bounds)/height;
    NSMutableArray *regions = [NSMutableArray new];
    for (int i = 0; i < count; i++) [regions addObject:[NSValue valueWithRect:NSMakeRect(rects[i].x*sx, rects[i].y*sy, rects[i].width*sx, rects[i].height*sy)]];
    state.regions = regions;
    state.directInput = count==1 && rects[0].x<=0 && rects[0].y<=0 && rects[0].width>=width && rects[0].height>=height;
    if (state.directInput) window.title = state.quickAdd?@"Sidelet · 快速添加":@"Sidelet · 快速操作";
    syncInputPanels(state);
    updatePointer(state);
}
static NSDictionary *rectJSON(NSRect rect) { return @{@"x":@(rect.origin.x),@"y":@(primaryTop()-NSMaxY(rect)),@"width":@(rect.size.width),@"height":@(rect.size.height)}; }
static NSDictionary *displayJSON(NSScreen *screen) {
    CGDirectDisplayID displayID = [screen.deviceDescription[@"NSScreenNumber"] unsignedIntValue];
    CFUUIDRef uuid = CGDisplayCreateUUIDFromDisplayID(displayID);
    NSString *identifier = uuid ? CFBridgingRelease(CFUUIDCreateString(kCFAllocatorDefault, uuid)) : [NSString stringWithFormat:@"display-%u",displayID];
    if (uuid) CFRelease(uuid);
    return @{@"id":identifier,@"workArea":rectJSON(screen.visibleFrame),@"bounds":rectJSON(screen.frame),@"scale":@1,@"backingScale":@(screen.backingScaleFactor),@"primary":displayID==CGMainDisplayID() ? @YES : @NO};
}
static char *jsonString(id value) { NSData *data = [NSJSONSerialization dataWithJSONObject:value options:0 error:nil]; return data ? strdup([[NSString alloc] initWithData:data encoding:NSUTF8StringEncoding].UTF8String) : NULL; }
char *SLFocusDiagnostic(void) {
    @autoreleasepool {
        NSRunningApplication *front=NSWorkspace.sharedWorkspace.frontmostApplication;
        NSWindow *key=NSApp.keyWindow;
        return jsonString(@{@"ownPID":@(getpid()),@"foregroundPID":@(front.processIdentifier),@"foregroundBundle":front.bundleIdentifier?:@"",
            @"appActive":@(NSApp.active),@"activationPolicy":@(NSApp.activationPolicy),@"mainWindowVisible":@(NSApp.mainWindow.visible),@"keyWindow":@(key?key.windowNumber:0),@"mainWindow":@(NSApp.mainWindow?NSApp.mainWindow.windowNumber:0),
            @"firstResponderClass":key.firstResponder?NSStringFromClass(key.firstResponder.class):@"",@"shortcutRegistered":@(keyboardShortcut!=NULL),@"quickAddShortcutRegistered":@(quickAddShortcut!=NULL),@"interactionTest":@(interactionTestObserver!=nil)});
    }
}
static void collectWebViews(NSView *view, NSMutableSet<NSValue *> *webviews) {
    if (!view) return;
    Class wk = NSClassFromString(@"WKWebView");
    if (wk && [view isKindOfClass:wk]) [webviews addObject:[NSValue valueWithNonretainedObject:view]];
    for (NSView *child in view.subviews) collectWebViews(child,webviews);
}
char *SLMemoryDiagnostic(void) {
    @autoreleasepool {
    if (!memoryStates) return jsonString(@{@"enabled":@NO});
    NSUInteger panels=0, observers=0, tracking=0, regions=0, quickAdds=0;
    for (SLWindowState *state in states.allValues) {
        panels += state.inputPanels.count; observers += state.observers.count; regions += state.regions.count;
        if(state.quickAdd) quickAdds++;
    }
    for (NSView *view in memoryInputViews.allObjects) tracking += view.trackingAreas.count;
    NSMutableSet *webviews = [NSMutableSet new];
    NSMutableArray *windows = [NSMutableArray new];
    for (NSWindow *window in NSApp.windows) {
        collectWebViews(window.contentView,webviews);
        [windows addObject:@{@"number":@(window.windowNumber),@"class":NSStringFromClass(window.class),@"visible":@(window.visible)}];
    }
    return jsonString(@{@"enabled":@YES,@"windowCount":@(NSApp.windows.count),@"windows":windows,
        @"attachedWebViews":@(webviews.count),@"boundStates":@(states.count),@"quickAddBound":@(quickAdds),@"liveStates":@(memoryStates.allObjects.count),
        @"ownedInputPanels":@(panels),@"liveInputPanels":@(memoryPanels.allObjects.count),@"liveInputViews":@(memoryInputViews.allObjects.count),
        @"inputPanelsCreated":@(memoryPanelsCreated),@"inputViewsCreated":@(memoryInputViewsCreated),
        @"inputTrackingAreas":@(tracking),@"regions":@(regions),@"windowObservers":@(observers),@"workspaceObservers":@(workspaceObservers.count),
        @"mouseMonitors":@((globalMouseMonitor?1:0)+(localMouseMonitor?1:0)),@"pendingFullscreenCheck":@(fullscreenCheck!=nil)});
    }
}
char *SLDisplays(void) { NSMutableArray *values = [NSMutableArray new]; for (NSScreen *screen in NSScreen.screens) [values addObject:displayJSON(screen)]; return jsonString(values); }
char *SLQuickAddDisplay(void) {
    NSScreen *target=NSScreen.screens.firstObject;
    for(NSScreen *screen in NSScreen.screens) if(NSPointInRect(NSEvent.mouseLocation,screen.frame)){target=screen;break;}
    return target?jsonString(displayJSON(target)):NULL;
}
char *SLDisplay(void *pointer) { NSScreen *screen = ((__bridge NSWindow *)pointer).screen ?: NSScreen.screens.firstObject; return screen ? jsonString(displayJSON(screen)) : NULL; }
char *SLDiagnostic(void *pointer) {
    NSWindow *window = (__bridge NSWindow *)pointer;
    SLWindowState *state = stateFor(window);
    NSMutableArray *routing = [NSMutableArray new];
    for (NSPanel *panel in state.inputPanels) {
        NSPoint center = NSMakePoint(NSMidX(panel.frame),NSMidY(panel.frame));
        [routing addObject:@{@"expectedWindow":@(panel.windowNumber),@"routedWindow":@([NSWindow windowNumberAtPoint:center belowWindowWithWindowNumber:0]),@"key":panel.isKeyWindow ? @YES : @NO,@"frame":rectJSON(panel.frame)}];
    }
    NSMutableDictionary *result = [@{@"class":NSStringFromClass(window.class),@"windowNumber":@(window.windowNumber),@"visible":window.visible ? @YES : @NO,@"key":window.isKeyWindow ? @YES : @NO,@"activeInput":state.active ? @YES : @NO,@"nonactivating":(window.styleMask & NSWindowStyleMaskNonactivatingPanel) ? @YES : @NO,@"ignoresMouse":window.ignoresMouseEvents ? @YES : @NO,@"regionCount":@(state.regions.count),@"inputMode":state.directInput ? @"direct" : @"region-panels",@"forwardedEvents":@(state.forwardedEvents),@"inputRouting":routing,@"frame":rectJSON(window.frame),@"viewportWidth":@(state.viewportWidth),@"viewportHeight":@(state.viewportHeight),@"webviewWidth":@(NSWidth(state.webview.bounds)),@"webviewHeight":@(NSHeight(state.webview.bounds))} mutableCopy];
    result[@"windowOpaque"] = @(window.opaque);
    result[@"windowBackgroundAlpha"] = @(window.backgroundColor.alphaComponent);
    if ([state.webview isKindOfClass:NSClassFromString(@"WKWebView")]) {
        @try { result[@"webviewDrawsBackground"] = [state.webview valueForKey:@"drawsBackground"]; }
        @catch (NSException *exception) { result[@"webviewBackgroundDiagnosticError"] = exception.reason ?: @"unsupported WebKit"; }
    }
    if (state.inputPanels.count >= 2) {
        NSRect a=state.inputPanels[0].frame, b=state.inputPanels[1].frame;
        if (NSMinY(a) > NSMaxY(b)) {
            NSPoint gap=NSMakePoint(NSMidX(a),(NSMinY(a)+NSMaxY(b))/2);
            result[@"gapRouting"] = @{@"x":@(gap.x),@"y":@(primaryTop()-gap.y),@"routedWindow":@([NSWindow windowNumberAtPoint:gap belowWindowWithWindowNumber:0])};
        }
    }
    return jsonString(result);
}
SLRect SLKeepQuickPointer(void *pointer, SLRect rect) {
    NSWindow *window = (__bridge NSWindow *)pointer;
    SLWindowState *state = stateFor(window);
    // Active cards stay open after editing; restore their task anchor rather
    // than following the pointer on the editor's footer when they shrink.
    if (state.active) return rect;
    NSPoint point = state.pointerScreen;
    if (window.visible && NSPointInRect(point, window.frame)) {
        double y = primaryTop() - point.y;
        // Keep the clicked footer inside the resized card, without moving it
        // when the pointer already fits. WorkArea clamping stays in Go.
        double inset = fmin(8, rect.height / 2);
        rect.y = fmax(y - rect.height + inset, fmin(rect.y, y - inset));
    }
    return rect;
}
SLRect SLClientOrigin(void *pointer) {
    NSWindow *window = (__bridge NSWindow *)pointer;
    NSView *view = stateFor(window).webview ?: window.contentView;
    NSRect rect = [window convertRectToScreen:[view convertRect:view.bounds toView:nil]];
    return (SLRect){NSMinX(rect),primaryTop()-NSMaxY(rect),NSWidth(rect),NSHeight(rect)};
}
int SLCaptureForeground(void) { return NSWorkspace.sharedWorkspace.frontmostApplication.processIdentifier; }
uintptr_t SLCaptureOwnWindow(void) {
    NSWindow *window = NSApp.keyWindow;
    return NSWorkspace.sharedWorkspace.frontmostApplication.processIdentifier == getpid() && window.visible && window.keyWindow ? (uintptr_t)(__bridge void *)window : 0;
}
bool SLRestoreForeground(int pid, uintptr_t ownWindow) {
    if (pid == getpid()) {
        // A captured main window may have been hidden while the overlay was
        // opened. Restoring focus must never reveal that hidden window.
        for (NSWindow *window in NSApp.windows) {
            if ((uintptr_t)(__bridge void *)window == ownWindow && window.visible && window.canBecomeKeyWindow) {
                [NSApp activateIgnoringOtherApps:YES];
                [window makeKeyAndOrderFront:nil];
                break;
            }
        }
        return true;
    }
    NSRunningApplication *app = [NSRunningApplication runningApplicationWithProcessIdentifier:pid];
    if (!app || app.terminated) return false;
    return [app activateWithOptions:NSApplicationActivateIgnoringOtherApps];
}
bool SLForegroundIsOurs(void) {
    if (NSWorkspace.sharedWorkspace.frontmostApplication.processIdentifier!=getpid()) return false;
    NSWindow *window = NSApp.keyWindow;
    if (window.isKeyWindow && stateFor(window).active) return true;
    for (SLWindowState *state in states.allValues) if (state.active && state.activationPending) return true;
    return false;
}
static BOOL coversFullscreenDisplay(CGRect bounds, CGRect display, NSEdgeInsets safe) {
    // A native fullscreen window on a notched Mac excludes the top safe area,
    // while a normal maximized window also stops above the Dock. Use screen
    // safe insets, never visibleFrame, to distinguish these cases.
    CGRect usable = CGRectMake(display.origin.x+safe.left,display.origin.y+safe.top,display.size.width-safe.left-safe.right,display.size.height-safe.top-safe.bottom);
    return usable.size.width > 0 && usable.size.height > 0 && CGRectContainsRect(CGRectInset(bounds,-1,-1),usable);
}
bool SLIsFullscreen(void) {
    NSRunningApplication *app = NSWorkspace.sharedWorkspace.frontmostApplication;
    if (!app || app.processIdentifier == getpid() || [app.bundleIdentifier isEqualToString:@"com.apple.finder"]) return false;
    NSArray *windows = CFBridgingRelease(CGWindowListCopyWindowInfo(kCGWindowListOptionOnScreenOnly | kCGWindowListExcludeDesktopElements, kCGNullWindowID));
    for (NSDictionary *info in windows) {
        if ([info[(id)kCGWindowOwnerPID] intValue] != app.processIdentifier || [info[(id)kCGWindowLayer] intValue] != 0) continue;
        CGRect bounds;
        if (!CGRectMakeWithDictionaryRepresentation((__bridge CFDictionaryRef)info[(id)kCGWindowBounds], &bounds)) continue;
        for (NSScreen *screen in NSScreen.screens) {
            NSDictionary *b = rectJSON(screen.frame);
            CGRect display = CGRectMake([b[@"x"] doubleValue],[b[@"y"] doubleValue],[b[@"width"] doubleValue],[b[@"height"] doubleValue]);
            if (coversFullscreenDisplay(bounds,display,screen.safeAreaInsets)) return true;
        }
    }
    return false;
}
void SLWatchForeground(uint64_t token) {
    SLStopWatching();
    foregroundWatchToken = token;
    workspaceObservers = [NSMutableArray new];
    for (NSString *name in @[NSWorkspaceDidActivateApplicationNotification,NSWorkspaceActiveSpaceDidChangeNotification]) {
        [workspaceObservers addObject:[NSWorkspace.sharedWorkspace.notificationCenter addObserverForName:name object:nil queue:NSOperationQueue.mainQueue usingBlock:^(NSNotification *note) {
            sideletNativeEvent(token, 4, 0, 0, false);
            requestFullscreenCheck([note.name isEqualToString:NSWorkspaceActiveSpaceDidChangeNotification] ? 0.8 : 0.15);
        }]];
    }
}
void SLStopWatching(void) {
    foregroundWatchToken = 0;
    if (fullscreenCheck) dispatch_block_cancel(fullscreenCheck);
    fullscreenCheck = nil;
    for (id observer in workspaceObservers) [NSWorkspace.sharedWorkspace.notificationCenter removeObserver:observer];
    workspaceObservers = nil;
}
