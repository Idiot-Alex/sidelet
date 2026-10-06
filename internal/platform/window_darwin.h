#pragma once
#include <stdbool.h>
#include <stdint.h>

typedef struct { double x, y, width, height; } SLRect;
bool SLBind(void *window, uint64_t id);
void SLClose(void *window);
void SLControlTheme(void *window, int theme);
bool SLControlRendering(void *window, bool visible);
void SLPassive(void *window);
bool SLActivate(void *window);
bool SLRegisterKeyboardShortcut(void *window);
bool SLRegisterQuickAddShortcut(void *window);
void SLConfigureQuickAdd(void *window);
bool SLEnableInteractionTest(void *window);
char *SLFocusDiagnostic(void);
void SLShow(void *window);
void SLHide(void *window);
bool SLMove(void *window, SLRect rect);
const char *SLLastError(void);
void SLRegions(void *window, const SLRect *rects, int count, double width, double height);
char *SLDisplays(void);
char *SLQuickAddDisplay(void);
char *SLDisplay(void *window);
char *SLDiagnostic(void *window);
void SLEnableMemoryDiagnostics(void);
char *SLMemoryDiagnostic(void);
SLRect SLClientOrigin(void *window);
int SLCaptureForeground(void);
uintptr_t SLCaptureOwnWindow(void);
bool SLRestoreForeground(int pid, uintptr_t ownWindow);
bool SLForegroundIsOurs(void);
bool SLIsFullscreen(void);
void SLWatchForeground(uint64_t id);
void SLStopWatching(void);
