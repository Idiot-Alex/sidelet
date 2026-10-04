<script lang="ts">
  import type { Snippet } from 'svelte';
  let { children }: { children?: Snippet } = $props();
</script>
<header>
  <span class="brand"><svg width="30" height="30" viewBox="0 0 30 30" fill="none" aria-hidden="true"><rect width="30" height="30" rx="9" fill="var(--accent)"/><path d="m7 15 4 4 8-9" stroke="var(--on-accent)" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/><path d="M24 8v4m0 3v3m0 3v2" stroke="var(--on-accent)" stroke-opacity=".55" stroke-width="2" stroke-linecap="round"/></svg><span>sidelet<span class="descriptor">留给眼前，回到手头。</span></span></span>
  <div class="actions">{@render children?.()}</div>
</header>
<style>
  header { display:flex; align-items:center; justify-content:space-between; gap:20px; min-height:74px; border-bottom:1px solid var(--line); }
  .brand { display:flex; align-items:center; gap:11px; color:var(--ink); font-size:19px; font-weight:650; letter-spacing:-.6px; }
  .descriptor { display:block; margin-top:2px; color:var(--muted); font-size:10px; font-weight:400; letter-spacing:.04em; }
  .actions { display:flex; align-items:center; gap:6px; }
  @media(max-width:700px) { .descriptor { display:none; } }
</style>
