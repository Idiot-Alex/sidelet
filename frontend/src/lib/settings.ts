export type Theme = 'mac' | 'paper' | 'graphite';
export const themes: { id: Theme; name: string; description: string }[] = [
  { id: 'mac', name: '精致 Mac', description: '柔和中性色，清爽专注' },
  { id: 'paper', name: '温暖纸色', description: '温润纸面，书写的节奏' },
  { id: 'graphite', name: '深色石墨', description: '低调深色，清晰有序' },
];
export interface Preferences {
  version: number;
  edge: { defaultSide: 'left' | 'right'; defaultDensity: 'compact' | 'normal' | 'relaxed' };
  startup: { enabled: boolean; showMainWindow: boolean };
  appearance: { theme: Theme; showDockIcon: boolean };
}
export interface SettingsState { appVersion: string; appBuild: string; value: Preferences; loginStatus: string; loginAvailable: boolean; error: string; quickAddShortcutError?: string }
export const loginLabels: Record<string, string> = { enabled: '已开启', notRegistered: '未开启', requiresApproval: '等待系统批准', notFound: '系统未找到此应用', unsupported: '当前平台暂不支持' };
export const notificationLabels: Record<string, string> = { authorized: '已允许', denied: '未允许', notDetermined: '尚未授权', unsupported: '当前平台暂不支持' };
