import { tick } from 'svelte';
import type { SettingsState } from './settings';
import type { Action, Snapshot } from './model';
import type { Rect } from './geometry';
import { memoryEnabled, memorySnapshot, trackMemory } from './memory';
import { measureQuickCard } from './quickCardSize';

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
let layoutRevision = 0;
export interface StackConfig { side: 'left' | 'right'; offset: number; stackIndex: number; stackCount: number; stackId: number; itemHeight: number; workHeight: number; viewportTop: number; layoutRevision: number }
export interface QuickAddState { open: boolean; saving: boolean; revision: number; resetVersion: number }
export interface PopupPreparation { view: 'add' | 'quick'; revision: number; sessionRevision: number }
let popupPreparation: PopupPreparation | undefined;
let quickRevision = 0;
export interface ParsedTask { title: string; dueAt: number }
export interface RequestResult { requestId: string; error: string; cancelled?: boolean; filename?: string; count?: number; parsed?: ParsedTask }
const pendingActions = new Map<string, { resolve: (result: RequestResult) => void; reject: (error: Error) => void }>();

export async function connect(onState: (state: Snapshot) => void, onMode: (mode: string) => void, onConfig: (config: StackConfig) => void, onError: (message: string) => void, onPresentation: (state: Presentation) => void, onPointer?: (state: NativePointer) => void, onNotification?: (state: NotificationStatus) => void, onSettings?: (state: SettingsState) => void, onSettingsOpen?: (open: boolean) => void, onQuickAdd?: (state: QuickAddState) => void, onPopup?: (state: PopupPreparation) => Promise<void>) {
  if (!native) return () => {};
  const { Events } = await import('@wailsio/runtime');
  let disposed = false;
  const cleanup = [
    Events.On('quick-add:state', event => { if (event.sender === windowName) onQuickAdd?.(event.data as QuickAddState); }),
 Events.On('settings:state', event => onSettings?.(event.data as SettingsState)),
 Events.On('settings:open', event => onSettingsOpen?.(!!event.data)),
    Events.On('reminder:status', event => onNotification?.(event.data as NotificationStatus)),
    Events.On('spike:state', event => onState(event.data as Snapshot)),
    Events.On('spike:mode', event => { if (event.sender === windowName) onMode(event.data as string); }),
    Events.On('spike:config', event => { if (event.sender === windowName) { layoutRevision = event.data.layoutRevision; onConfig(event.data); } }),
    Events.On('spike:measure', event => {
      if (event.sender !== windowName || role === 'control') return;
      const revision = layoutRevision;
      const popup = popupPreparation;
      void tick().then(() => {
        if (disposed || revision !== layoutRevision || popup !== popupPreparation) return;
        hitRegions(Array.from(document.querySelectorAll('[data-hit]')).map(element => {
          const rect = element.getBoundingClientRect();
          return { x: rect.x, y: rect.y, width: rect.width, height: rect.height };
        }), revision);
      });
    }),
    Events.On('spike:error', event => { if (!event.sender || event.sender === windowName) onError(String(event.data)); }),
    Events.On('spike:presentation', event => onPresentation(event.data as Presentation)),
    Events.On('spike:pointer', event => { if (event.sender === windowName) onPointer?.(event.data as NativePointer); }),
    Events.On('todo:result', event => {
      if (event.sender !== windowName) return;
      const result = event.data as RequestResult;
      const pending = pendingActions.get(result.requestId);
      if (!pending) return;
      pendingActions.delete(result.requestId);
      if (result.error) pending.reject(new Error(result.error)); else pending.resolve(result);
    }),
  ];
  if (role === 'popup') {
    cleanup.push(Events.On('popup:prepare', event => {
      if (event.sender !== windowName) return;
      const value = event.data as PopupPreparation;
      if (popupPreparation && value.revision <= popupPreparation.revision) return;
      popupPreparation = value;
      quickRevision = value.view === 'quick' ? value.sessionRevision : 0;
      void (onPopup ? onPopup(value) : tick()).then(() => {
        if (!disposed && popupPreparation === value) {
          hitRegions([{ x: 0, y: 0, width: innerWidth, height: innerHeight }]);
          send('popup-rendered', { revision: value.sessionRevision, cardHeight: value.view === 'quick' ? measureQuickCard() : undefined });
        }
      }).catch(cause => onError(String(cause)));
    }));
  }
  if (role === 'quick' || role === 'popup') {
    cleanup.push(Events.On('quick:prepare', event => {
      if (event.sender !== windowName) return;
      quickRevision = event.data;
      const revision = quickRevision;
      void tick().then(() => { if (!disposed && quickRevision === revision) send('quick-rendered', { revision, cardHeight: measureQuickCard() }); });
    }));
    if (new URLSearchParams(location.search).get('trace-quick') === '1') {
      cleanup.push(Events.On('quick:shown', event => {
        if (event.sender !== windowName) return;
        requestAnimationFrame(() => { if (!disposed) send('quick-painted', { revision: event.data }); });
      }));
    }
  }
  const disposeSubscriptions = memoryEnabled ? cleanup.map(() => trackMemory('bridgeSubscription')) : [];
  if (memoryEnabled) {
    cleanup.push(Events.On('spike:memory-request', event => {
      send('memory-view', { label: event.data, metric: memorySnapshot() });
    }));
    disposeSubscriptions.push(trackMemory('bridgeSubscription'));
  }
  send('ready');
  return () => { disposed = true; cleanup.forEach(off => off()); disposeSubscriptions.forEach(off => off()); pendingActions.forEach(pending => pending.reject(new Error('窗口已关闭，尚未确认保存结果。'))); pendingActions.clear(); };
}

export function send(type: string, payload: Record<string, unknown> = {}) {
  (window as InvokeWindow)._wails?.invoke(JSON.stringify({ type, ...payload, ...popupPacket() }));
}
export function reportQuickSize(cardHeight: number) {
  if (quickRevision) send('quick-size', { revision: quickRevision, cardHeight });
}
export function quickSessionRevision() { return quickRevision; }
function popupPacket() {
  return role === 'popup' ? { popupView: popupPreparation?.view ?? 'quick', popupRevision: popupPreparation?.revision ?? 0 } : {};
}
export async function dispatch(action: Action): Promise<void> { await request('action', { action }); }
export function request(type: string, data: Record<string, unknown> = {}): Promise<RequestResult> {
  const requestId = `${windowName}-${++requestSequence}`;
  return new Promise((resolve, reject) => {
    if (!(window as InvokeWindow)._wails?.invoke) { reject(new Error('尚未连接到应用，请稍后重试。')); return; }
    const payload = JSON.stringify({ type, ...data, requestId, ...popupPacket() });
    if (new TextEncoder().encode(payload).length > 65536) { reject(new Error('内容过长，请缩短备注后重试。')); return; }
    pendingActions.set(requestId, { resolve, reject });
    try { (window as InvokeWindow)._wails!.invoke(payload); }
    catch (error) { pendingActions.delete(requestId); reject(error); }
  });
}
export function hitRegions(rects: Rect[], revision = layoutRevision) { send('regions', { rects, viewportWidth: innerWidth, viewportHeight: innerHeight, revision }); }
