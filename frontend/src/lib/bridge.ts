import type { SettingsState } from './settings';
import type { Action, Snapshot } from './model';
import type { Rect } from './geometry';
import { memoryEnabled, memorySnapshot, trackMemory } from './memory';

export const role = new URLSearchParams(location.search).get('view') ?? 'lab';
export const native = role !== 'lab';
export const hostPlatform = new URLSearchParams(location.search).get('platform') ?? 'windows';
export const shortcutLabel = hostPlatform === 'darwin' ? 'Ctrl Option T' : 'Ctrl Alt T';
export const fixtureMode = new URLSearchParams(location.search).get('spike') === '1';
const windowName = role === 'stack' ? `stack-${new URLSearchParams(location.search).get('index') ?? '0'}` : role;
type InvokeWindow = Window & { _wails?: { invoke: (message: string) => void } };
export interface Presentation { quickOpen: boolean; sourceIndex: number; todoId: number; quiet: boolean; arranging: boolean }
export interface NotificationStatus { authorization: string; message: string }
export interface NativePointer { x: number; y: number; inside: boolean }
let requestSequence = 0;
const pendingActions = new Map<string, { resolve: () => void; reject: (error: Error) => void }>();

export async function connect(onState: (state: Snapshot) => void, onMode: (mode: string) => void, onConfig: (config: { side: 'left' | 'right'; offset: number; stackIndex: number; stackCount: number; stackId: number; itemHeight: number }) => void, onError: (message: string) => void, onPresentation: (state: Presentation) => void, onPointer?: (state: NativePointer) => void, onNotification?: (state: NotificationStatus) => void, onSettings?: (state: SettingsState) => void, onSettingsOpen?: (open: boolean) => void) {
  if (!native) return () => {};
  const { Events } = await import('@wailsio/runtime');
  const cleanup = [
 Events.On('settings:state', event => onSettings?.(event.data as SettingsState)),
 Events.On('settings:open', event => onSettingsOpen?.(!!event.data)),
    Events.On('reminder:status', event => onNotification?.(event.data as NotificationStatus)),
    Events.On('spike:state', event => onState(event.data as Snapshot)),
    Events.On('spike:mode', event => { if (event.sender === windowName) onMode(event.data as string); }),
    Events.On('spike:config', event => { if (event.sender === windowName) onConfig(event.data); }),
    Events.On('spike:measure', event => {
      if (event.sender !== windowName || role === 'control') return;
      hitRegions(Array.from(document.querySelectorAll('[data-hit]')).map(element => {
        const rect = element.getBoundingClientRect();
        return { x: rect.x, y: rect.y, width: rect.width, height: rect.height };
      }));
    }),
    Events.On('spike:error', event => { if (!event.sender || event.sender === windowName) onError(String(event.data)); }),
    Events.On('spike:presentation', event => onPresentation(event.data as Presentation)),
    Events.On('spike:pointer', event => { if (event.sender === windowName) onPointer?.(event.data as NativePointer); }),
    Events.On('todo:result', event => {
      if (event.sender !== windowName) return;
      const result = event.data as { requestId: string; error: string };
      const pending = pendingActions.get(result.requestId);
      if (!pending) return;
      pendingActions.delete(result.requestId);
      if (result.error) pending.reject(new Error(result.error)); else pending.resolve();
    }),
  ];
  const disposeSubscriptions = memoryEnabled ? cleanup.map(() => trackMemory('bridgeSubscription')) : [];
  if (memoryEnabled) {
    cleanup.push(Events.On('spike:memory-request', event => {
      send('memory-view', { label: event.data, metric: memorySnapshot() });
    }));
    disposeSubscriptions.push(trackMemory('bridgeSubscription'));
  }
  send('ready');
  return () => { cleanup.forEach(off => off()); disposeSubscriptions.forEach(off => off()); pendingActions.forEach(pending => pending.reject(new Error('窗口已关闭，尚未确认保存结果。'))); pendingActions.clear(); };
}

export function send(type: string, payload: Record<string, unknown> = {}) {
  (window as InvokeWindow)._wails?.invoke(JSON.stringify({ type, ...payload }));
}
export function dispatch(action: Action): Promise<void> { return request('action', { action }); }
export function request(type: string, data: Record<string, unknown> = {}): Promise<void> {
  const requestId = `${windowName}-${++requestSequence}`;
  return new Promise((resolve, reject) => {
    if (!(window as InvokeWindow)._wails?.invoke) { reject(new Error('尚未连接到应用，请稍后重试。')); return; }
    const payload = JSON.stringify({ type, ...data, requestId });
    if (new TextEncoder().encode(payload).length > 65536) { reject(new Error('内容过长，请缩短备注后重试。')); return; }
    pendingActions.set(requestId, { resolve, reject });
    try { (window as InvokeWindow)._wails!.invoke(payload); }
    catch (error) { pendingActions.delete(requestId); reject(error); }
  });
}
export function hitRegions(rects: Rect[]) { send('regions', { rects, viewportWidth: innerWidth, viewportHeight: innerHeight }); }
