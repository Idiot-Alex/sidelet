<script lang="ts">
  import type { Snippet } from 'svelte';
  let { title, count, children }: { title: string; count?: number; children?: Snippet } = $props();
</script>
<header>
  <h1>{title}{#if count !== undefined}<span class="count">{count}</span>{/if}</h1>
  <div class="actions">{@render children?.()}</div>
</header>
<style>
  header { display:flex; align-items:center; justify-content:space-between; gap:20px; min-height:64px; border-bottom:1px solid var(--line); }
  h1 { display:flex; align-items:center; gap:10px; margin:0; font:var(--heading-weight) 21px/1.3 var(--font-heading); letter-spacing:-.025em; white-space:nowrap; }
  .count { padding:3px 7px; border-radius:5px; background:var(--accent-soft); color:var(--accent); font:500 11px/1.3 var(--font-ui); font-variant-numeric:tabular-nums; letter-spacing:0; }
  .actions { display:flex; align-items:center; gap:6px; }
</style>
