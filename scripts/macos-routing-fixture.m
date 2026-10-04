// Interactive fixture: operate with Computer Use or a physical mouse.
// No synthetic input is posted by this program.
#import <WebKit/WebKit.h>
#import "../internal/platform/window_darwin.m"

static FILE *fixtureLog;
static NSPanel *renderPanel;
static int forwardedClicks;
void sideletNativeEvent(uint64_t token, int kind, double x, double y, bool inside) {
    if (kind == 1) {
        fprintf(fixtureLog,"presence inside=%d x=%.1f y=%.1f\n",inside,x,y);
        fflush(fixtureLog);
    }
}
// Accessibility is enabled only in this separate fixture process so an
// app-scoped UI tool can explicitly target the real native receiving window.
static BOOL accessibleHelper(id object, SEL selector) { return YES; }

@interface RoutingWebPanel : NSPanel
@property(strong) WKWebView *webView;
@end
@implementation RoutingWebPanel
- (BOOL)isAccessibilityElement { return NO; }
@end

@interface FixtureDelegate : NSObject <NSApplicationDelegate, WKNavigationDelegate, WKScriptMessageHandler>
@property(strong) WKWebView *webView;
@end
@implementation FixtureDelegate
- (void)applicationDidFinishLaunching:(NSNotification *)note {
    class_replaceMethod(SLInputPanel.class,@selector(isAccessibilityElement),(IMP)accessibleHelper,"B@:");
    NSRect area = NSScreen.screens.firstObject.visibleFrame;
    NSRect frame = NSMakeRect(NSMinX(area)+360,NSMidY(area)-120,300,240);
    RoutingWebPanel *panel = [[RoutingWebPanel alloc] initWithContentRect:frame styleMask:NSWindowStyleMaskBorderless | NSWindowStyleMaskNonactivatingPanel backing:NSBackingStoreBuffered defer:NO];
    panel.releasedWhenClosed = NO; renderPanel = panel;
    WKWebViewConfiguration *config = [WKWebViewConfiguration new];
    [config.userContentController addScriptMessageHandler:self name:@"routing"];
    self.webView = [[WKWebView alloc] initWithFrame:NSMakeRect(0,0,300,240) configuration:config];
    [self.webView setValue:@NO forKey:@"drawsBackground"];
    self.webView.navigationDelegate = self;
    panel.webView = self.webView; panel.contentView = self.webView;
    if (!SLBind((__bridge void *)panel,1)) { fprintf(fixtureLog,"FAIL bind\n"); exit(1); }
    NSString *page = @"<!doctype html><meta charset=utf-8><style>body{margin:0;background:transparent}#tag{position:absolute;left:280px;top:12px;width:20px;height:44px;background:#e9ece5;cursor:pointer;user-select:none}#tag.expanded{left:100px;width:200px}</style><div id=tag></div><script>const tag=document.querySelector('#tag');let clicks=0;for(const name of ['pointerenter','mousedown','mouseup','click','dblclick']){tag.addEventListener(name,e=>{window.webkit.messageHandlers.routing.postMessage({type:name,x:e.clientX,y:e.clientY,detail:e.detail,trusted:e.isTrusted,button:e.button,shift:e.shiftKey,meta:e.metaKey});if(name==='click'){tag.classList.add('expanded');tag.textContent='native forwarded clicks: '+(++clicks);window.webkit.messageHandlers.routing.postMessage({type:'expand'});}});}</script>";
    [self.webView loadHTMLString:page baseURL:nil];
}
- (void)webView:(WKWebView *)view didFinishNavigation:(WKNavigation *)navigation {
    SLRect rect = {280,12,20,44};
    SLRegions((__bridge void *)renderPanel,&rect,1,300,240); SLShow((__bridge void *)renderPanel);
    [self describeHelper];
    fprintf(fixtureLog,"READY collapsed helper; click screenshot center (10,22)\n"); fflush(fixtureLog);
}
- (void)describeHelper {
    NSPanel *helper = stateFor(renderPanel).inputPanels.firstObject;
    helper.title = @"Sidelet · native routing fixture";
    [helper.contentView setAccessibilityRole:NSAccessibilityGroupRole];
    [helper.contentView setAccessibilityLabel:@"Native task input rectangle"];
}
- (void)userContentController:(WKUserContentController *)controller didReceiveScriptMessage:(WKScriptMessage *)message {
    NSDictionary *body = message.body;
    if ([body[@"type"] isEqual:@"expand"]) {
        SLRect rect = {100,12,200,44}; SLRegions((__bridge void *)renderPanel,&rect,1,300,240);
        [self describeHelper]; return;
    }
    NSData *json = [NSJSONSerialization dataWithJSONObject:body options:0 error:nil];
    SLWindowState *state = stateFor(renderPanel);
    if ([body[@"type"] isEqual:@"click"]) forwardedClicks++;
    fprintf(fixtureLog,"web=%s forwarded=%lu renderKey=%d helperKey=%d foregroundPID=%d clicks=%d\n",
        [[NSString alloc] initWithData:json encoding:NSUTF8StringEncoding].UTF8String,
        (unsigned long)state.forwardedEvents,renderPanel.isKeyWindow,state.inputPanels.firstObject.isKeyWindow,
        NSWorkspace.sharedWorkspace.frontmostApplication.processIdentifier,forwardedClicks);
    fflush(fixtureLog);
}
@end
int main(int argc, const char **argv) {
    if (argc != 2) { fprintf(stderr,"usage: routing-fixture log-file\n"); return 2; }
    fixtureLog = fopen(argv[1],"w"); if (!fixtureLog) return 1;
    @autoreleasepool {
        NSApplication *app = NSApplication.sharedApplication;
        [app setActivationPolicy:NSApplicationActivationPolicyAccessory];
        FixtureDelegate *delegate = [FixtureDelegate new]; app.delegate = delegate;
        [app run];
    }
    fclose(fixtureLog); return 0;
}
