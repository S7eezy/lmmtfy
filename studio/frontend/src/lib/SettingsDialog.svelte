<script>
  // Mapping settings, from the column editor. The target has its own step,
  // and a linked target file its own editor: this dialog only says which
  // target the mapping uses and leads there.
  import Modal from './Modal.svelte'
  import { Settings2, Target, Lock, FileCog, Package, FilePlus2, PencilRuler } from '@lucide/svelte'
  import Section from './Section.svelte'
  import SettingsForm from './SettingsForm.svelte'
  import OutputForm from './OutputForm.svelte'
  import { outputSummary } from './mapping.js'
  let { mapping = $bindable(), targetMode, targetPath = '', onclose, ontargetstep, onedittarget } = $props()
  const target = $derived(mapping.target.inline)
  const fileName = (p) => p.split(/[\\/]/).pop()
  const info = $derived({
    file: [Lock, `Linked target file ${fileName(targetPath)}`, 'Its columns and file format are changed in the target editor.'],
    new: [FilePlus2, `New target file ${targetPath ? fileName(targetPath) : ''}`, 'Written with the mapping on the next save, then linked like any target file.'],
    inline: [Package, 'Target inside this mapping', 'The mapping file holds its target.'],
  }[targetMode])
  const Icon = $derived(info[0])
</script>

<Modal title="Mapping settings" icon={Settings2} {onclose} wide>
  <SettingsForm bind:mapping />
  <Section icon={Target} title="Target">
    <div class="row target">
      <span class="icon"><Icon size={16} /></span>
      <span class="what"><strong>{target.name || info[1]}</strong>{#if target.name}<span class="muted"> · {info[1]}</span>{/if}<span class="muted desc">{info[2]} {outputSummary(target.output)}</span></span>
      <span class="spacer"></span>
      {#if targetMode === 'file'}<button onclick={onedittarget}><PencilRuler size={13} /> Edit target…</button>{/if}
      <button onclick={ontargetstep}><FileCog size={13} /> Change the target…</button>
    </div>
    {#if targetMode !== 'file'}
      <h3>Output file format</h3>
      <OutputForm output={target.output} />
    {/if}
  </Section>
  {#snippet footer()}<button class="primary" onclick={onclose}>Done</button>{/snippet}
</Modal>

<style>
  .target { gap: 10px; align-items: center; }
  .target button { display: inline-flex; gap: 4px; align-items: center; white-space: nowrap; }
  .icon { display: inline-flex; width: 30px; height: 30px; border-radius: 8px; align-items: center; justify-content: center; background: var(--neutral-soft); color: var(--muted); flex-shrink: 0; }
  .what { display: flex; flex-direction: column; min-width: 0; }
  .desc { font-size: 12px; }
</style>
