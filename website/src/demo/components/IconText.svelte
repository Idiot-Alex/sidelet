<script lang="ts">
  import type { ComponentProps, Snippet } from 'svelte';
  import Icon from './Icon.svelte';
  let { name, size = 16, gap = 6, children }: {
    name: ComponentProps<typeof Icon>['name'];
    size?: number;
    gap?: number;
    children: Snippet;
  } = $props();
</script>

<span class="icon-text" style:--icon-gap={`${gap}px`}>
  <Icon {name} {size} /><span class="label">{@render children()}</span>
</span>

<style>
  .icon-text { display:inline-flex; align-items:center; gap:var(--icon-gap); vertical-align:middle; line-height:1.333333; }
  .label { display:block; min-width:0; }
</style>
