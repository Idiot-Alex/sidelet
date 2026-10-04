#pragma once
char *SLNotificationState(void);
char *SLNotificationDeliver(const char *identifier,const char *title,const char *body);
char *SLNotificationRemove(const char *identifiers);
void SLNotificationStart(void);
void SLNotificationRequestPermission(void);
void SLNotificationStop(void);
