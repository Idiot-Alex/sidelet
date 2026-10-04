#import <AppKit/AppKit.h>
#import <ServiceManagement/ServiceManagement.h>
#include "settings_darwin.h"
#include <string.h>
// Called on the UI thread. Accessory bootstrap avoids a Dock flash when the
// saved preference is hidden. Switching to Regular here does not activate the
// application or reveal any windows during background startup.
int SLSetDockVisible(int visible, void *control) {
 NSApplicationActivationPolicy policy = visible ? NSApplicationActivationPolicyRegular : NSApplicationActivationPolicyAccessory;
 if (NSApp.activationPolicy == policy) return 1;
 NSWindow *window = (__bridge NSWindow *)control;
 BOOL wasVisible = window.visible;
 BOOL wasKey = window.keyWindow && NSApp.active;
 if (![NSApp setActivationPolicy:policy]) return 0;
 // Regular -> Accessory can hide the application. Keep an already-visible
 // settings window in place, while background startup stays nonactivating.
 if (wasVisible) {
  if (NSApp.hidden) [NSApp unhideWithoutActivation];
  [window orderFrontRegardless];
  if (wasKey) { [NSApp activateIgnoringOtherApps:YES]; [window makeKeyWindow]; }
 }
 return NSApp.activationPolicy == policy ? 1 : 0;
}
char *SLLoginStatus(void) {
 @autoreleasepool {
  switch (SMAppService.mainAppService.status) {
   case SMAppServiceStatusEnabled: return strdup("enabled");
   case SMAppServiceStatusRequiresApproval: return strdup("requiresApproval");
   case SMAppServiceStatusNotRegistered: return strdup("notRegistered");
   default: return strdup("notFound");
  }
 }
}
char *SLSetLogin(int enabled) {
 @autoreleasepool {
  SMAppService *service = SMAppService.mainAppService;
  BOOL registered = service.status == SMAppServiceStatusEnabled || service.status == SMAppServiceStatusRequiresApproval;
  if ((enabled && registered) || (!enabled && service.status == SMAppServiceStatusNotRegistered)) return NULL;
  NSError *error = nil;
  BOOL ok = enabled ? [service registerAndReturnError:&error] : [service unregisterAndReturnError:&error];
  return ok ? NULL : strdup(error.localizedDescription.UTF8String ?: "Login item change failed");
 }
}
// NSWorkspace calls run on the UI thread. Notification pane URL is best effort;
// fall back to opening System Settings when the OS cannot resolve it.
int SLOpenSettings(int login) {
 @autoreleasepool {
  if (login) { [SMAppService openSystemSettingsLoginItems]; return 1; }
  NSURL *url = [NSURL URLWithString:@"x-apple.systempreferences:com.apple.Notifications-Settings.extension"];
  if ([NSWorkspace.sharedWorkspace openURL:url]) return 1;
  return [NSWorkspace.sharedWorkspace openURL:[NSURL fileURLWithPath:@"/System/Applications/System Settings.app"]];
 }
}
