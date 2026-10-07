// Keep persisted values stable: old “较高” (1) joins “重要” (2).
export const priorityOptions = [
  { value: 0, label: '普通' },
  { value: 2, label: '重要' },
  { value: 3, label: '紧急' },
] as const;

export function normalizePriority(value = 0): 0 | 2 | 3 {
  return value === 3 ? 3 : value === 1 || value === 2 ? 2 : 0;
}

export function priorityLabel(value?: number): string {
  return priorityOptions.find(option => option.value === normalizePriority(value))!.label;
}
