<script>
  // Step 2 of a new mapping, and the Target step of any mapping: what the
  // file becomes. Three ways, as cards:
  //   file    an existing target file, linked and locked: the mapping adapts
  //           to it, columns with matching names are mapped right away;
  //   new     a target defined here and saved as its own file on the first
  //           save (the default);
  //   inline  a target kept inside the mapping file, for one-offs.
  // The target defined here (the draft, drafted from the file's columns, or
  // a copy of the current target) is kept while trying target files, so
  // switching between cards loses nothing.
  import { FolderOpen, FilePlus2, FileCog, Lock, ArrowLeft, ArrowRight, CircleAlert, Check, Clock, Package, Target } from '@lucide/svelte'
  import * as api from './api.js'
  import * as M from './mapping.js'
  import { toast } from './toast.svelte.js'
  import OutputForm from './OutputForm.svelte'
  import Section from './Section.svelte'

  let {
    mapping = $bindable(), mode = $bindable(), targetPath = $bindable(),
    draft, copy = false, sources = [], detection = null, recent = [], preset = '',
    onlinked = () => {}, onpreset = () => {}, onback = null, backLabel = '', oncontinue, continueLabel = 'Next',
  } = $props()

  let view = $state(mode)
  const fileName = (p) => p.split(/[\\/]/).pop()
  const cols = $derived(M.columns(mapping))
  const missing = $derived(cols.filter((c) => !mapping.columns?.[c.name]))
  const others = $derived(recent.filter((r) => r.path !== targetPath))
  const target = $derived(mapping.target.inline)
  const defaultFile = $derived(`${M.targetName(target.name || mapping.name || 'target').replaceAll('_', '-')}.target.yaml`)

  // The draft follows edits made while the target is defined here.
  function keepDraft() {
    if (mode === 'file') return
    draft.target = M.clone(mapping.target.inline)
    draft.columns = M.clone(mapping.columns ?? {})
  }

  async function link(path) {
    try {
      const t = await api.loadTarget(path)
      keepDraft()
      const rules = { ...draft.columns, ...(mode === 'file' ? mapping.columns : {}) }
      const detect = (s) => s && detection?.columns.find((c) => (s.index ? c.index === s.index : c.name === s.name))
      const matched = M.linkTarget(mapping, t, rules, sources, detect)
      mode = view = 'file'
      targetPath = path
      api.remember('target', path)
      const left = t.columns.filter((c) => !mapping.columns[c.name]).length
      onlinked({ matched })
      toast(left ? 'info' : 'ok', `${t.name || fileName(path)}: ${t.columns.length - left} of ${t.columns.length} columns found in this file${left ? `, ${left} to map` : ''}.`)
    } catch (e) {
      toast('error', String(e?.message ?? e))
    }
  }
  async function browse() {
    const path = await api.chooseFile('Use a target file', 'target')
    if (path) link(path)
  }
  function chooseFile() {
    view = 'file'
    if (mode !== 'file' && !recent.length) browse()
  }
  function own(m) {
    if (mode === 'file') {
      mapping.target.inline = M.clone(draft.target)
      mapping.columns = M.clone(draft.columns)
      // A copy of a target file is another target: named so, it is not
      // saved over the original.
      if (copy) mapping.target.inline.name = `${draft.target.name || 'target'}-copy`
      targetPath = ''
      onlinked({ matched: [] })
    }
    if (m === 'inline' || mode === 'file') targetPath = ''
    mode = view = m
  }
  async function chooseLocation() {
    const path = await api.chooseSaveFile('Save the new target as', targetPath || defaultFile, 'target')
    if (path) targetPath = path
  }

  // Started from a target ("New mapping for this target"): link it.
  $effect(() => {
    if (preset) {
      link(preset)
      onpreset()
    }
  })
</script>

<div class="page panel scroll">
  <div class="intro">
    <h2><Target size={18} /> What does this file become?</h2>
    <p class="muted">The target is the format the receiving software expects: its columns, in order, and its file format.</p>
  </div>

  <div class="cards">
    <button class="choice card" class:on={view === 'file'} onclick={chooseFile}>
      <span class="icon"><FolderOpen size={20} /></span>
      <span><span class="title">Use a target file</span>
        <span class="desc">The format is fixed: this file is mapped onto it. Columns with matching names are mapped right away.</span></span>
    </button>
    <button class="choice card" class:on={view === 'new'} onclick={() => own('new')}>
      <span class="icon"><FilePlus2 size={20} /></span>
      <span><span class="title">New target file {#if mode !== 'file'}<span class="badge">recommended</span>{/if}</span>
        <span class="desc">{copy ? 'A copy of the current target, saved as a new file you can change.' : 'Built from this file’s columns, saved as its own file: later mappings can reuse it.'}</span></span>
    </button>
    <button class="choice card" class:on={view === 'inline'} onclick={() => own('inline')}>
      <span class="icon"><Package size={20} /></span>
      <span><span class="title">Inside this mapping</span>
        <span class="desc">One-off: the target lives in the mapping file. It can be moved to its own file later.</span></span>
    </button>
  </div>

  {#if view === 'file'}
    {#if mode === 'file'}
      <Section icon={Lock} title={target.name || fileName(targetPath)} hint="Linked: names, types, order and file format come from this file. Change them with Edit target, in the column editor.">
        <div class="facts">
          <span class="mono" title={targetPath}><FileCog size={13} /> {fileName(targetPath)}</span>
          <span>{cols.length} columns, {cols.filter((c) => c.required).length} required</span>
          <span>{M.outputSummary(target.output)}</span>
        </div>
        {#if !missing.length}
          <p class="fit good"><Check size={15} /> Every target column has a source in this file.</p>
        {:else}
          <p class="fit"><strong>{cols.length - missing.length} of {cols.length}</strong> target columns found in this file by name. Still to map:</p>
          <div class="missing">
            {#each missing.slice(0, 40) as c}
              <span class="chip" class:required={c.required} title={c.required ? 'Required' : 'Optional: left empty if not mapped'}>{#if c.required}<CircleAlert size={12} />{/if}{c.name}</span>
            {/each}
            {#if missing.length > 40}<span class="muted">and {missing.length - 40} more</span>{/if}
          </div>
          <p class="hint">Map them in the next steps: the Review list shows each one.</p>
        {/if}
      </Section>
    {/if}
    <Section icon={Clock} title={mode === 'file' ? 'Use another target file' : 'Choose a target file'}>
      <div class="recent">
        {#each others as r}
          <button class="link item" title={r.path} onclick={() => link(r.path)}>
            <FileCog size={15} /><span class="name">{fileName(r.path)}</span><span class="muted mono dir">{r.path.slice(0, -fileName(r.path).length)}</span>
          </button>
        {/each}
        <button onclick={browse}><FolderOpen size={14} /> Browse…</button>
      </div>
    </Section>
  {:else}
    <Section icon={view === 'new' ? FilePlus2 : Package} title={view === 'new' ? 'New target file' : 'Target inside the mapping'}
      hint={copy ? 'A copy of the target: rename, retype, reorder or add its columns in Map columns.' : 'Its columns are this file’s columns: rename, retype, reorder or add them in Map columns.'}>
      <div class="grid two">
        <label>Target name <input value={target.name ?? ''} oninput={(e) => (target.name = e.target.value)} placeholder={mapping.name || 'e.g. erp-orders'} /></label>
        {#if view === 'new'}
          <label>Saved as
            <span class="row location">
              <span class="mono" title={targetPath}>{targetPath ? fileName(targetPath) : defaultFile}</span>
              <span class="muted">{targetPath ? '' : 'next to the mapping, on the first save'}</span>
              <button class="link" onclick={chooseLocation}>Choose…</button>
            </span>
          </label>
        {/if}
      </div>
    </Section>
    <Section icon={FileCog} title="Output file format" hint="How converted files are written.">
      <OutputForm output={target.output} />
    </Section>
  {/if}

  <div class="row actions">
    {#if onback}<button onclick={onback}><ArrowLeft size={14} /> {backLabel}</button>{/if}
    <span class="spacer"></span>
    {#if view === 'file' && mode !== 'file'}<span class="muted">Choose a target file, or another option.</span>{/if}
    <button class="primary" onclick={oncontinue} disabled={view === 'file' && mode !== 'file'}>{continueLabel} <ArrowRight size={14} /></button>
  </div>
</div>

<style>
  .page { max-width: 960px; margin: 0 auto; height: 100%; padding: 6px 24px 18px; }
  .intro h2 { font-size: 17px; margin: 14px 0 2px; display: flex; gap: 8px; align-items: center; }
  .cards { display: grid; grid-template-columns: repeat(3, 1fr); gap: 10px; margin: 16px 0 6px; }
  .card { padding: 14px; }
  .card .icon { display: inline-flex; width: 36px; height: 36px; flex-shrink: 0; border-radius: 9px; align-items: center; justify-content: center; background: var(--soft); color: var(--accent); }
  .card.on .icon { background: var(--accent); color: var(--accent-text); }
  .card .title { display: flex; gap: 6px; align-items: center; margin-bottom: 2px; }
  .two { grid-template-columns: 1fr 1fr; }
  .location { gap: 8px; min-height: 30px; color: var(--text); }
  .facts { display: flex; flex-wrap: wrap; gap: 6px 18px; font-size: 12px; margin-bottom: 8px; }
  .facts span { display: inline-flex; gap: 4px; align-items: center; }
  .fit { display: flex; gap: 6px; align-items: center; margin: 8px 0 6px; }
  .fit.good { color: var(--high); }
  .missing { display: flex; flex-wrap: wrap; gap: 5px; }
  .chip { display: inline-flex; gap: 3px; align-items: center; font-family: var(--mono); font-size: 12px; padding: 1px 8px; border-radius: 10px; background: var(--warn-soft); color: var(--warn); }
  .chip.required { background: var(--error-soft); color: var(--error); }
  .recent { display: grid; gap: 2px; justify-items: start; }
  .recent > button:last-child { margin-top: 6px; display: inline-flex; gap: 5px; align-items: center; }
  .item { display: flex; gap: 8px; align-items: center; width: 100%; padding: 5px 2px; color: var(--text); text-align: left; }
  .item :global(svg) { color: var(--accent); flex-shrink: 0; }
  .item .name { font-weight: 600; }
  .item .dir { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; direction: rtl; flex: 1; text-align: left; }
  .actions { position: sticky; bottom: -18px; background: var(--panel); padding: 12px 0; border-top: 1px solid var(--line); margin-top: 8px; }
  .actions button { display: inline-flex; gap: 6px; align-items: center; }
</style>
