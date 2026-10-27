<script>
  // Detected type and how many sampled values fit it, coloured by level.
  import TypeIcon from './TypeIcon.svelte'
  import { confidence, pct } from './confidence.js'
  // compact: type icon and share only, the full label in the tooltip.
  let { detected, bar = false, compact = false } = $props()
  const c = $derived(confidence(detected))
  const titles = {
    high: 'Every sampled value fits this type',
    good: 'Almost every sampled value fits this type',
    fair: 'Most sampled values fit this type: check the rejected ones',
    low: 'Text that almost always reads as another type: check the data or the type',
    text: 'Plain text',
  }
</script>

{#if c && bar}
  <div class="bar {c.level}" title={titles[c.level]}><div style="width: {Math.round(c.share * 100)}%"></div></div>
{:else if c}
  <span class="conf {c.level}" title="{c.label}: {titles[c.level]}">
    <TypeIcon type={c.type} size={12} />
    {#if !compact}<span>{c.label}</span>{:else if c.level !== 'text'}<span>{pct(c.share)}</span>{/if}
  </span>
{/if}

<style>
  .conf { display: inline-flex; gap: 4px; align-items: center; border-radius: 10px; padding: 1px 8px; font-size: 11px; font-weight: 600; white-space: nowrap; }
  .conf :global(.type-icon) { color: inherit; }
  .high { background: var(--high-soft); color: var(--high); }
  .good { background: var(--good-soft); color: var(--good); }
  .fair { background: var(--fair-soft); color: var(--fair); }
  .low { background: var(--low-soft); color: var(--low); }
  .text { background: var(--neutral-soft); color: var(--muted); }
  .bar { height: 6px; border-radius: 3px; background: var(--neutral-soft); overflow: hidden; margin-top: 6px; }
  .bar > div { height: 100%; border-radius: 3px; }
  .bar.high > div { background: var(--high); }
  .bar.good > div { background: var(--good); }
  .bar.fair > div { background: var(--fair); }
  .bar.low > div { background: var(--low); }
  .bar.text > div { background: var(--line); }
</style>
