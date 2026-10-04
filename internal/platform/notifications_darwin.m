#import <Cocoa/Cocoa.h>
#import <UserNotifications/UserNotifications.h>
#include "notifications_darwin.h"

extern void sideletNotificationEvent(char *kind,char *identifier);
static char *encoded(id value) {
 NSData *data=[NSJSONSerialization dataWithJSONObject:value options:0 error:nil];
 return strdup([[NSString alloc] initWithData:data encoding:NSUTF8StringEncoding].UTF8String);
}
static char *failure(NSString *message) {return encoded(@{@"error":message ?: @"Notification service failed"});}
static void event(NSString *kind,NSString *identifier) {sideletNotificationEvent((char *)kind.UTF8String,(char *)(identifier ?: @"").UTF8String);}

@interface SLNotificationDelegate : NSObject<UNUserNotificationCenterDelegate>
@end
@implementation SLNotificationDelegate
- (void)userNotificationCenter:(UNUserNotificationCenter *)center willPresentNotification:(UNNotification *)notification withCompletionHandler:(void (^)(UNNotificationPresentationOptions))completionHandler {
 completionHandler(UNNotificationPresentationOptionBanner|UNNotificationPresentationOptionList|UNNotificationPresentationOptionSound);
}
- (void)userNotificationCenter:(UNUserNotificationCenter *)center didReceiveNotificationResponse:(UNNotificationResponse *)response withCompletionHandler:(void (^)(void))completionHandler {
 if ([response.actionIdentifier isEqualToString:UNNotificationDefaultActionIdentifier]) event(@"open",response.notification.request.identifier);
 completionHandler();
}
@end
static SLNotificationDelegate *notificationDelegate;
static id wakeObserver,clockObserver;
void SLNotificationStart(void) {
 @try {
  notificationDelegate=[SLNotificationDelegate new];
  UNUserNotificationCenter.currentNotificationCenter.delegate=notificationDelegate;
  wakeObserver=[NSWorkspace.sharedWorkspace.notificationCenter addObserverForName:NSWorkspaceDidWakeNotification object:nil queue:NSOperationQueue.mainQueue usingBlock:^(NSNotification *n){event(@"refresh",@"");}];
  clockObserver=[NSNotificationCenter.defaultCenter addObserverForName:NSSystemClockDidChangeNotification object:nil queue:NSOperationQueue.mainQueue usingBlock:^(NSNotification *n){event(@"refresh",@"");}];
 } @catch(NSException *exception) { NSLog(@"Sidelet notifications unavailable: %@",exception.reason); }
}
void SLNotificationStop(void) {
 if(wakeObserver)[NSWorkspace.sharedWorkspace.notificationCenter removeObserver:wakeObserver];
 if(clockObserver)[NSNotificationCenter.defaultCenter removeObserver:clockObserver];
 wakeObserver=nil;clockObserver=nil;
}
void SLNotificationRequestPermission(void) {
 @try {
  [UNUserNotificationCenter.currentNotificationCenter requestAuthorizationWithOptions:UNAuthorizationOptionAlert|UNAuthorizationOptionSound completionHandler:^(BOOL granted,NSError *error){event(@"refresh",@"");}];
 } @catch(NSException *exception) {event(@"refresh",@"");}
}
// These synchronous wrappers are called only by the Go queue worker, never on
// AppKit's main thread. Completion blocks retain their state after a timeout.
char *SLNotificationState(void) {
 @autoreleasepool { @try {
  UNUserNotificationCenter *center=UNUserNotificationCenter.currentNotificationCenter;
  dispatch_group_t group=dispatch_group_create();
  __block NSString *authorization=@"notDetermined";
  __block NSArray *pending=@[],*delivered=@[];
  dispatch_group_enter(group);
  [center getNotificationSettingsWithCompletionHandler:^(UNNotificationSettings *settings){
   switch(settings.authorizationStatus){
    case UNAuthorizationStatusAuthorized: case UNAuthorizationStatusProvisional: authorization=@"authorized";break;
    case UNAuthorizationStatusDenied: authorization=@"denied";break;
    default: authorization=@"notDetermined";break;
   }
   dispatch_group_leave(group);
  }];
  dispatch_group_enter(group);
  [center getPendingNotificationRequestsWithCompletionHandler:^(NSArray<UNNotificationRequest *> *requests){pending=[requests valueForKey:@"identifier"];dispatch_group_leave(group);}];
  dispatch_group_enter(group);
  [center getDeliveredNotificationsWithCompletionHandler:^(NSArray<UNNotification *> *notifications){NSMutableArray *ids=[NSMutableArray new];for(UNNotification *n in notifications)[ids addObject:n.request.identifier];delivered=ids;dispatch_group_leave(group);}];
  if(dispatch_group_wait(group,dispatch_time(DISPATCH_TIME_NOW,5*NSEC_PER_SEC)))return failure(@"Notification service timed out");
  return encoded(@{@"authorization":authorization,@"pending":pending,@"delivered":delivered});
 } @catch(NSException *exception){return failure(exception.reason);} }
}
char *SLNotificationDeliver(const char *identifier,const char *title,const char *body) {
 @autoreleasepool { @try {
  UNMutableNotificationContent *content=[UNMutableNotificationContent new];
  content.title=[NSString stringWithUTF8String:title];content.body=[NSString stringWithUTF8String:body];content.sound=UNNotificationSound.defaultSound;
  UNNotificationRequest *request=[UNNotificationRequest requestWithIdentifier:[NSString stringWithUTF8String:identifier] content:content trigger:nil];
  dispatch_semaphore_t done=dispatch_semaphore_create(0);__block NSError *error=nil;
  [UNUserNotificationCenter.currentNotificationCenter addNotificationRequest:request withCompletionHandler:^(NSError *e){error=e;dispatch_semaphore_signal(done);}];
  if(dispatch_semaphore_wait(done,dispatch_time(DISPATCH_TIME_NOW,5*NSEC_PER_SEC)))return failure(@"Notification delivery timed out");
  return error?failure(error.localizedDescription):encoded(@{});
 } @catch(NSException *exception){return failure(exception.reason);} }
}
char *SLNotificationRemove(const char *identifiers) {
 @autoreleasepool { @try {
  NSData *data=[[NSString stringWithUTF8String:identifiers] dataUsingEncoding:NSUTF8StringEncoding];
  NSArray *ids=[NSJSONSerialization JSONObjectWithData:data options:0 error:nil];
  UNUserNotificationCenter *center=UNUserNotificationCenter.currentNotificationCenter;
  [center removePendingNotificationRequestsWithIdentifiers:ids];
  [center removeDeliveredNotificationsWithIdentifiers:ids];
  return encoded(@{});
 } @catch(NSException *exception){return failure(exception.reason);} }
}
