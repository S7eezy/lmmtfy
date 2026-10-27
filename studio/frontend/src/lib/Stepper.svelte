<script>
  // The steps of building a mapping. Done steps can be revisited.
  import { FileSearch, Target, Settings2, Columns3, Check } from '@lucide/svelte'
  let { current, enabled = [true, true, true, true], onstep } = $props()
  const steps = [
    { id: 'read', label: 'Read the file', icon: FileSearch },
    { id: 'target', label: 'Target', icon: Target },
    { id: 'setup', label: 'Settings', icon: Settings2 },
    { id: 'edit', label: 'Map columns', icon: Columns3 },
  ]
  const index = $derived(steps.findIndex((s) => s.id === current))
</script>

<nav class="stepper" aria-label="Steps">
  {#each steps as s, i}
    {@const Icon = i < index ? Check : s.icon}
    {#if i > 0}<span class="sep"></span>{/if}
    <button class="step" class:current={i === index} class:done={i < index} disabled={!enabled[i] || i === index} title={s.label}
      onclick={() => onstep(s.id)} aria-current={i === index ? 'step' : undefined}>
      <span class="dot"><Icon size={13} /></span><span class="label">{s.label}</span>
    </button>
  {/each}
</nav>

<style>
  .stepper { display: flex; align-items: center; gap: 4px; }
  .step { display: inline-flex; align-items: center; gap: 6px; border: none; background: none; padding: 4px 8px; color: var(--muted); }
  .step:disabled { opacity: 1; cursor: default; }
  .step:not(:disabled):hover { color: var(--accent); }
  .dot { display: inline-flex; width: 22px; height: 22px; border-radius: 50%; align-items: center; justify-content: center; border: 1px solid var(--line); }
  .current { color: var(--text); font-weight: 600; }
  .current .dot { background: var(--accent); border-color: var(--accent); color: var(--accent-text); }
  .done .dot { background: var(--high-soft); border-color: var(--high); color: var(--high); }
  .sep { width: 18px; height: 1px; background: var(--line); }
  /* Narrow windows: only the current step keeps its label. */
  @media (max-width: 1360px) {
    .step:not(.current) .label { display: none; }
    .sep { width: 10px; }
  }
</style>
