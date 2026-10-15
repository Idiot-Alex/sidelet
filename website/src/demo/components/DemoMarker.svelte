<script lang="ts">
  let { compact = false }: { compact?: boolean } = $props();
</script>
<span class:compact aria-hidden="true">{#if !compact}桌面任务{/if}</span>
<style>
  span { display: flex; align-items: center; width: 100%; height: 36px; background: var(--surface-alt); color: var(--ink); border: 1px solid var(--line); border-radius: 5px; padding: 0 10px; font-size: 12px; }
  .compact { position: relative; flex: 0 0 14px; align-self: stretch; justify-content: center; width: 14px; height: auto; padding: 0; border: 0; border-radius: 0; color: var(--accent); background: none; }
  .compact::before { content: ''; width: 3px; height: 23px; border-radius: 2px; background: currentColor; }
</style>
