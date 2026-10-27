<script>
  // Step 2 of a new mapping: name it and choose its target before mapping
  // columns, so neither is forgotten.
  import { ArrowLeft, ArrowRight } from '@lucide/svelte'
  import SettingsForm from './SettingsForm.svelte'
  let { mapping = $bindable(), targetPath = $bindable(), sources = [], onback, oncontinue } = $props()
  const ready = $derived(!!mapping.name?.trim())
  let form = $state(null)
  $effect(() => {
    // Start in the name field, text selected: the proposed name is a guess.
    const input = form?.querySelector('input')
    input?.focus()
    input?.select()
  })
</script>

<div class="page panel scroll" bind:this={form}>
  <div class="intro">
    <h2>Name the mapping and choose its target</h2>
    <p class="muted">Everything here can be changed later in Settings.</p>
  </div>
  <SettingsForm bind:mapping bind:targetPath {sources} />
  <div class="row actions">
    <button onclick={onback}><ArrowLeft size={14} /> Reading settings</button>
    <span class="spacer"></span>
    {#if !ready}<span class="muted">Give the mapping a name to continue.</span>{/if}
    <button class="primary" onclick={oncontinue} disabled={!ready}>Map columns <ArrowRight size={14} /></button>
  </div>
</div>

<style>
  .page { max-width: 920px; margin: 0 auto; height: 100%; padding: 6px 24px 18px; }
  .intro h2 { font-size: 17px; margin: 14px 0 2px; }
  .actions { position: sticky; bottom: -18px; background: var(--panel); padding: 12px 0; border-top: 1px solid var(--line); }
  .actions button { display: inline-flex; gap: 6px; align-items: center; }
</style>
