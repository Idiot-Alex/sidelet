<script lang="ts">
  let { name, size = 16 }: { name: 'check' | 'plus' | 'settings' | 'layout' | 'moon' | 'hide' | 'back' | 'pin' | 'close' | 'list'; size?: number } = $props();
  const paths = {
    check: 'm5 12 4 4L19 6', plus: 'M12 5v14M5 12h14',
    settings: 'M4 7h16M4 17h16M8 4v6M16 14v6',
    layout: 'M4 4h16v16H4zM15 4v16M18 8h-1M18 12h-1M18 16h-1',
    moon: 'M20 14a8 8 0 0 1-10-10 8 8 0 1 0 10 10Z',
    hide: 'M4 5h16v14H4zM8 9l-3 3 3 3M5 12h9',
    back: 'm10 5-7 7 7 7M3 12h18', pin: 'M9 3h6l-1 6 4 4v2H6v-2l4-4-1-6ZM12 15v6',
    close: 'm6 6 12 12M6 18 18 6', list: 'M9 6h11M9 12h11M9 18h11M4 6h.01M4 12h.01M4 18h.01',
  };
</script>
<svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d={paths[name]} /></svg>
<style>svg { flex-shrink: 0; vertical-align: middle; }</style>
