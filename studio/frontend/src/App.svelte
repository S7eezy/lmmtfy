<script>
  // lmmtfy Studio. A new mapping goes through three steps: read the file,
  // name the mapping and choose its target, map the columns. An opened
  // mapping goes straight to the columns. The mapping being edited lives
  // here; children edit it through bindings, and previews are refreshed
  // shortly after each change.
  import { FilePlus2, FolderOpen, Settings2, Play, Package, Save, FileSpreadsheet, FileCog, Clock, Upload, Lightbulb } from '@lucide/svelte'
  import * as api from './lib/api.js'
  import { sources as sourcesOf } from './lib/mapping.js'
  import { toast } from './lib/toast.svelte.js'
  import ReadStep from './lib/ReadStep.svelte'
  import SetupStep from './lib/SetupStep.svelte'
  import Editor from './lib/Editor.svelte'
  import Stepper from './lib/Stepper.svelte'
  import Toasts from './lib/Toasts.svelte'
  import SettingsDialog from './lib/SettingsDialog.svelte'
  import ConvertDialog from './lib/ConvertDialog.svelte'
  import ExportDialog from './lib/ExportDialog.svelte'
  import Modal from './lib/Modal.svelte'

  let screen = $state('home') // home | read | setup | edit
  let isNew = $state(false) // a new mapping, not saved yet: its setup step is still ahead
  let csvPath = $state('')
  let mapping = $state(null)
  let detection = $state(null)
  let mappingPath = $state('')
  let targetPath = $state('')
  let saved = $state('') // the mapping as last saved or opened
  let raw = $state(null)
  let preview = $state(null)
  let issues = $state([])
  let dialog = $state(null)
  let confirmText = $state(null)
  let confirmResolve = null
  let version = $state('')
  let recent = $state([])
  let dropping = $state(false)

  // Compared with sorted keys: removing and re-adding an option moves it
  // in the object without changing the mapping.
  const canonical = (v) => JSON.stringify(v, (_, x) => (x && typeof x === 'object' && !Array.isArray(x)
    ? Object.fromEntries(Object.entries(x).filter(([, y]) => y !== undefined).sort(([a], [b]) => a.localeCompare(b))) : x))
  const dirty = $derived(mapping !== null && canonical(mapping) !== saved)
  const sources = $derived(sourcesOf(raw, mapping?.input?.header ?? true))
  const errorCount = $derived(issues.filter((i) => i.severity === 'error').length)
  const fileName = (p) => p.split(/[\\/]/).pop()

  api.version().then((v) => (version = v))
  const loadRecent = () => api.recent().then((r) => (recent = r ?? [])).catch(() => {})
  loadRecent()

  // The window asks before closing with unsaved changes (see app.go).
  $effect(() => { api.setDirty(dirty) })

  // Dropping a file on the window opens it: mappings by extension, anything
  // else as a CSV file.
  api.onFileDrop((paths) => {
    dropping = false
    const path = paths?.[0]
    if (!path) return
    if (/\.ya?ml$/i.test(path)) openMapping(path)
    else openCSV(path)
  })

  const fail = (e) => toast('error', String(e?.message ?? e))
  function ask(text) {
    confirmText = text
    return new Promise((resolve) => (confirmResolve = resolve))
  }
  function answer(yes) {
    confirmText = null
    confirmResolve?.(yes)
  }
  async function leaveOk() {
    return !dirty || (await ask('The mapping has unsaved changes. Discard them?'))
  }

  function keydown(e) {
    if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 's' && screen === 'edit') {
      e.preventDefault()
      if (errorCount === 0) save(e.shiftKey)
      else toast('error', 'Fix the errors before saving.')
    }
  }

  // Raw preview while reading; output preview and checks while editing.
  $effect(() => {
    if (screen !== 'read' || !mapping || !csvPath) return
    const input = JSON.parse(JSON.stringify(mapping.input))
    const path = csvPath
    const t = setTimeout(() => api.readRaw(path, input).then((r) => (raw = r)).catch(fail), 200)
    return () => clearTimeout(t)
  })
  $effect(() => {
    if (screen !== 'edit' || !mapping) return
    // Read everything the preview depends on here, synchronously: Svelte
    // only re-runs an effect for what it reads before the first await or
    // timeout.
    const snapshot = JSON.parse(JSON.stringify(mapping))
    const path = csvPath
    const t = setTimeout(async () => {
      try {
        if (path) {
          const res = await api.preview(path, snapshot)
          preview = res
          issues = res.issues ?? []
        } else {
          issues = await api.check(snapshot)
        }
      } catch (e) { fail(e) }
    }, 300)
    return () => clearTimeout(t)
  })

  async function openCSV(path = '') {
    if (!(await leaveOk())) return
    path ||= await api.chooseFile('Open a CSV file', 'csv')
    if (!path) return
    try {
      const o = await api.open(path)
      csvPath = path
      detection = o.detection
      mapping = o.mapping
      mappingPath = ''
      targetPath = ''
      saved = ''
      raw = null
      preview = null
      isNew = true
      screen = 'read'
      api.remember('csv', path).then(loadRecent)
    } catch (e) { fail(e) }
  }

  async function redetect() {
    try {
      const o = await api.open(csvPath)
      detection = o.detection
      mapping.input = o.mapping.input
      toast('ok', 'Reading settings detected again.')
    } catch (e) { fail(e) }
  }

  async function openMapping(path = '') {
    if (!(await leaveOk())) return
    path ||= await api.chooseFile('Open a mapping', 'mapping')
    if (!path) return
    try {
      const l = await api.loadMapping(path)
      mapping = l.mapping
      mappingPath = path
      targetPath = l.target_path ?? ''
      saved = canonical(mapping)
      detection = null
      preview = null
      raw = null
      csvPath = ''
      isNew = false
      screen = 'edit'
      api.remember('mapping', path).then(loadRecent)
      toast('info', `Opened ${fileName(path)}. Open a sample CSV file to preview conversions.`)
    } catch (e) { fail(e) }
  }

  async function openSample() {
    const path = await api.chooseFile('Sample CSV file for the preview', 'csv')
    if (!path) return
    try {
      raw = await api.readRaw(path, JSON.parse(JSON.stringify(mapping.input)))
      csvPath = path
      // Detection only informs the column panel (detected types, examples):
      // the mapping's own reading settings are kept.
      api.open(path).then((o) => (detection = o.detection)).catch(() => {})
      api.remember('csv', path).then(loadRecent)
    } catch (e) { fail(e) }
  }

  async function save(as = false) {
    let path = mappingPath
    if (!path || as) {
      const base = csvPath ? csvPath.replace(/[^\\/]+$/, '') : ''
      path = await api.chooseSaveFile('Save the mapping', base + (mapping.name || 'mapping') + '.mapping.yaml', 'mapping')
      if (!path) return
    }
    try {
      await api.saveMapping(mapping, path, targetPath)
      mappingPath = path
      saved = canonical(mapping)
      isNew = false
      api.remember('mapping', path).then(loadRecent)
      toast('ok', `Saved ${fileName(path)}${targetPath ? ` and ${fileName(targetPath)}` : ''}.`)
    } catch (e) { fail(e) }
  }

  async function home() {
    if (await leaveOk()) {
      screen = 'home'
      mapping = null
      saved = ''
    }
  }

  function goto(step) {
    if (step === 'read' && !csvPath) return
    screen = step
  }
</script>

<svelte:window onkeydown={keydown} ondragenter={() => (dropping = true)} ondragleave={(e) => { if (!e.relatedTarget) dropping = false }} ondrop={() => (dropping = false)} />

<div class="app">
  <header class="bar">
    <button class="link brand" onclick={home} title="Home">
      <img src="/icon.png" alt="" width="22" height="22" />
      lmmtfy <span>Studio</span>
    </button>
    {#if mapping}
      <span class="title">
        <FileCog size={15} />
        <strong>{mapping.name || 'untitled'}</strong>
        {#if dirty}<span class="unsaved" title="Unsaved changes">●</span>{/if}
      </span>
      <span class="divider"></span>
      <Stepper current={screen} enabled={[!!csvPath, true, !isNew || screen === 'edit']} onstep={goto} />
      <span class="spacer"></span>
      {#if screen === 'edit'}
        <button onclick={() => (dialog = 'settings')} title="Mapping settings"><Settings2 size={14} /> Settings</button>
        <button onclick={() => (dialog = 'convert')} disabled={errorCount > 0} title="Convert a file"><Play size={14} /> Convert</button>
        <button onclick={() => (dialog = 'export')} disabled={errorCount > 0} title="Build standalone converters"><Package size={14} /> Export</button>
        <button class="primary" onclick={() => save()} disabled={errorCount > 0} title="Save (Ctrl/⌘ S, with Shift: save as)"><Save size={14} /> Save</button>
      {/if}
    {:else}
      <span class="spacer"></span>
    {/if}
  </header>

  <main>
    {#if screen === 'home'}
      <div class="home">
        <img class="logo" src="/icon.png" alt="" width="72" height="72" />
        <h1>Let Me Map That For You</h1>
        <p class="muted lead">Describe once how a messy CSV file becomes the format your software expects. Then convert files here, from the command line, or with a standalone converter.</p>
        <div class="cards">
          <button class="card" onclick={() => openCSV()}>
            <span class="card-icon"><FilePlus2 size={22} /></span>
            <strong>New mapping from a CSV file</strong>
            <span>Open a file, check how it is read, name the mapping, map its columns.</span>
          </button>
          <button class="card" onclick={() => openMapping()}>
            <span class="card-icon"><FolderOpen size={22} /></span>
            <strong>Open a mapping</strong>
            <span>Edit a saved mapping, convert files, export converters.</span>
          </button>
        </div>
        <p class="muted drop-hint"><Upload size={14} /> or drop a CSV or mapping file anywhere on this window</p>
        {#if recent.length}
          <div class="recent panel">
            <h3><Clock size={13} /> Recent</h3>
            {#each recent as r}
              <button class="link item" title={r.path} onclick={() => (r.kind === 'mapping' ? openMapping(r.path) : openCSV(r.path))}>
                {#if r.kind === 'mapping'}<FileCog size={15} />{:else}<FileSpreadsheet size={15} />{/if}
                <span class="name">{fileName(r.path)}</span>
                <span class="muted mono dir">{r.path.slice(0, -fileName(r.path).length)}</span>
              </button>
            {/each}
          </div>
        {/if}
        <p class="muted small"><Lightbulb size={12} /> Tip: <kbd>Ctrl</kbd>/<kbd>⌘</kbd> <kbd>S</kbd> saves the mapping. Version {version}</p>
      </div>
    {:else if screen === 'read' && mapping}
      <ReadStep bind:input={mapping.input} {raw} path={csvPath} notes={detection?.notes ?? []} onredetect={redetect}
        continueLabel={isNew ? 'Next: mapping & target' : 'Back to the columns'} oncontinue={() => (screen = isNew ? 'setup' : 'edit')} />
    {:else if screen === 'setup' && mapping}
      <SetupStep bind:mapping bind:targetPath {sources} onback={() => (screen = 'read')} oncontinue={() => (screen = 'edit')} />
    {:else if screen === 'edit' && mapping}
      <Editor bind:mapping {sources} {detection} {preview} {issues} samplePath={csvPath} onsample={openSample} />
    {/if}
  </main>
</div>

{#if dropping}
  <div class="dropzone"><Upload size={36} /><span>Drop a CSV file or a mapping to open it</span></div>
{/if}

{#if dialog === 'settings'}
  <SettingsDialog bind:mapping bind:targetPath {sources} onclose={() => (dialog = null)} />
{:else if dialog === 'convert'}
  <ConvertDialog {mapping} input={csvPath} onclose={() => (dialog = null)} />
{:else if dialog === 'export'}
  <ExportDialog {mapping} onclose={() => (dialog = null)} />
{/if}

{#if confirmText}
  <Modal title="Unsaved changes" onclose={() => answer(false)}>
    <p>{confirmText}</p>
    {#snippet footer()}
      <button onclick={() => answer(false)}>Cancel</button>
      <button class="primary" onclick={() => answer(true)}>Discard changes</button>
    {/snippet}
  </Modal>
{/if}

<Toasts />

<style>
  .app { display: flex; flex-direction: column; height: 100%; }
  .bar { display: flex; align-items: center; gap: 8px; padding: 7px 14px; border-bottom: 1px solid var(--line); background: var(--panel); flex-wrap: wrap; }
  .bar button { display: inline-flex; gap: 5px; align-items: center; }
  .brand { font-weight: 700; font-size: 15px; color: var(--text); gap: 7px !important; }
  .brand span { color: var(--accent); font-weight: 500; }
  .title { display: inline-flex; gap: 6px; align-items: center; margin-left: 6px; color: var(--muted); }
  .title strong { color: var(--text); max-width: 220px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .unsaved { color: var(--warn); font-size: 12px; }
  .divider { width: 1px; height: 20px; background: var(--line); margin: 0 4px; }
  .sample { display: inline-flex; gap: 5px; align-items: center; color: var(--muted); max-width: 220px; overflow: hidden; white-space: nowrap; text-overflow: ellipsis; }
  main { flex: 1; min-height: 0; padding: 12px; }
  .home { max-width: 760px; margin: 5vh auto 0; text-align: center; }
  .logo { border-radius: 16px; }
  .home h1 { font-size: 26px; margin: 10px 0 6px; }
  .lead { max-width: 600px; margin: 0 auto; }
  .cards { display: grid; grid-template-columns: 1fr 1fr; gap: 14px; margin: 26px 0 10px; }
  .card { display: flex; flex-direction: column; gap: 6px; text-align: left; padding: 18px; border-radius: 12px; transition: transform 0.12s, box-shadow 0.12s; }
  .card:hover { transform: translateY(-2px); box-shadow: 0 8px 24px rgb(0 0 0 / 0.12); }
  .card-icon { display: inline-flex; width: 40px; height: 40px; border-radius: 10px; align-items: center; justify-content: center; background: var(--soft); color: var(--accent); margin-bottom: 4px; }
  .card strong { font-size: 15px; }
  .card span:last-child { color: var(--muted); }
  .drop-hint { display: inline-flex; gap: 6px; align-items: center; }
  .small { font-size: 11px; display: inline-flex; gap: 4px; align-items: center; margin-top: 16px; }
  .recent { text-align: left; padding: 6px 14px 10px; margin: 14px 0 0; }
  .recent h3 { display: flex; gap: 5px; align-items: center; }
  .item { display: flex; gap: 8px; align-items: center; width: 100%; padding: 5px 2px; color: var(--text); text-align: left; }
  .item :global(svg) { color: var(--accent); flex-shrink: 0; }
  .item .name { font-weight: 600; }
  .item .dir { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; direction: rtl; flex: 1; text-align: left; }
  .dropzone { position: fixed; inset: 10px; border: 3px dashed var(--accent); border-radius: 16px; background: color-mix(in srgb, var(--bg) 88%, transparent); display: flex; flex-direction: column; gap: 12px; align-items: center; justify-content: center; color: var(--accent); font-size: 16px; font-weight: 600; z-index: 25; pointer-events: none; }
</style>
