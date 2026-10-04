#import <AppKit/AppKit.h>
#import <ServiceManagement/ServiceManagement.h>
#include "settings_darwin.h"
#include <string.h>
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
