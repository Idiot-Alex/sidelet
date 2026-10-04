// Render our own WKWebView fixture, not a screen capture or input synthesizer.
#import <WebKit/WebKit.h>
#import "../internal/platform/window_darwin.m"

void sideletNativeEvent(uint64_t token, int kind, double x, double y, bool inside) {}
static int failures;
static NSString *output;

@interface TransparencyPanel : NSPanel
@property(strong) WKWebView *webView;
@end
@implementation TransparencyPanel
@end

static void check(BOOL passed, NSString *name) {
    printf("%s %s\n", passed ? "PASS" : "FAIL", name.UTF8String);
    if (!passed) failures++;
}

@interface TransparencyDelegate : NSObject <NSApplicationDelegate, WKNavigationDelegate>
@property(strong) TransparencyPanel *panel;
@property NSInteger themeIndex;
@end
@implementation TransparencyDelegate
- (void)applicationDidFinishLaunching:(NSNotification *)notification {
    NSRect rect=NSMakeRect(300,300,312,240);
    self.panel=[[TransparencyPanel alloc] initWithContentRect:rect styleMask:NSWindowStyleMaskBorderless | NSWindowStyleMaskNonactivatingPanel backing:NSBackingStoreBuffered defer:NO];
    self.panel.releasedWhenClosed=NO;
    self.panel.webView=[[WKWebView alloc] initWithFrame:NSMakeRect(0,0,312,240)];
    self.panel.webView.navigationDelegate=self;
    self.panel.contentView=self.panel.webView;
    // Start with the stock WKWebView background, just like production Wails.
    check([[self.panel.webView valueForKey:@"drawsBackground"] boolValue],@"fixture starts with the default WebKit backing enabled");
    check(SLBind((__bridge void *)self.panel,1),@"bind actual WKWebView overlay");
    check(![[self.panel.webView valueForKey:@"drawsBackground"] boolValue],@"overlay disables WebKit page backing");
    check(!self.panel.opaque && self.panel.backgroundColor.alphaComponent==0 && !self.panel.hasShadow,@"native overlay has no background or shadow");
    SLShow((__bridge void *)self.panel);
    [self loadTheme];
    dispatch_after(dispatch_time(DISPATCH_TIME_NOW,20*NSEC_PER_SEC),dispatch_get_main_queue(),^{fprintf(stderr,"FAIL transparency render timed out\n");exit(1);});
}
- (void)loadTheme {
    NSString *css=[NSString stringWithContentsOfFile:@"frontend/src/style.css" encoding:NSUTF8StringEncoding error:nil];
    NSString *themeCSS=[NSString stringWithContentsOfFile:@"frontend/src/theme.css" encoding:NSUTF8StringEncoding error:nil];
    if (!css || !themeCSS) { fprintf(stderr,"Run from the Sidelet repository root\n"); exit(1); }
    NSString *theme=@[@"mac",@"paper",@"graphite"][self.themeIndex];
    NSString *html=[NSString stringWithFormat:@"<!doctype html><html data-theme='%@'><style>%@ %@ .card{position:absolute;left:20px;top:20px;width:100px;height:44px;border-radius:8px;background:var(--surface)}</style><div id='app'><div class='native-stack'><div class='card'></div></div></div></html>",theme,css,themeCSS];
    [self.panel.webView loadHTMLString:html baseURL:nil];
}
- (void)webView:(WKWebView *)view didFinishNavigation:(WKNavigation *)navigation {
    // Give WebKit a compositor turn; snapshot includes only this fixture's view.
    dispatch_after(dispatch_time(DISPATCH_TIME_NOW,200*NSEC_PER_MSEC),dispatch_get_main_queue(),^{
        WKSnapshotConfiguration *config=[WKSnapshotConfiguration new];
        config.rect=view.bounds;
        [view takeSnapshotWithConfiguration:config completionHandler:^(NSImage *image,NSError *error) {
            if (!image || error) { fprintf(stderr,"FAIL snapshot: %s\n",error.description.UTF8String);exit(1); }
            NSBitmapImageRep *bitmap=[NSBitmapImageRep imageRepWithData:image.TIFFRepresentation];
            double scale=bitmap.pixelsWide/312.0;
            CGFloat blank=[bitmap colorAtX:200*scale y:100*scale].alphaComponent;
            CGFloat content=[bitmap colorAtX:60*scale y:40*scale].alphaComponent;
            CGFloat corner=[bitmap colorAtX:20*scale y:20*scale].alphaComponent;
            printf("Theme %ld alpha blank=%.4f content=%.4f rounded-corner=%.4f\n",(long)self.themeIndex,blank,content,corner);
            check(blank<.01,@"empty overlay pixels are fully transparent");
            check(content>.99,@"task content remains opaque and legible");
            check(corner<.01,@"empty rounded corner has no page backing");
            NSData *png=[bitmap representationUsingType:NSBitmapImageFileTypePNG properties:@{}];
            NSString *path=self.themeIndex==0 ? output : [NSString stringWithFormat:@"%@-%@.png",output.stringByDeletingPathExtension,@[@"mac",@"paper",@"graphite"][self.themeIndex]];
            check([png writeToFile:path atomically:YES],@"save reproducible alpha-channel fixture");
            if (++self.themeIndex < 3) { [self loadTheme]; return; }
            SLClose((__bridge void *)self.panel); [self.panel close];
            printf("Transparency assertions: %d failures\n",failures);
            exit(failures ? 1 : 0);
        }];
    });
}
@end
int main(int argc,const char **argv) {
    @autoreleasepool {
        output=argc>1 ? @(argv[1]) : @"build/results/macos-transparency.png";
        NSApplication *app=NSApplication.sharedApplication;
        [app setActivationPolicy:NSApplicationActivationPolicyAccessory];
        TransparencyDelegate *delegate=[TransparencyDelegate new];app.delegate=delegate;
        [app run];
    }
    return 1;
}
