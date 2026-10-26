<script>
  // lmmtfy Studio: home → read the file → map columns. The mapping being
  // edited lives here; children edit it through bindings, and previews are
  // refreshed shortly after each change.
  import * as api from './lib/api.js'
  import { sources as sourcesOf } from './lib/mapping.js'
  import ReadStep from './lib/ReadStep.svelte'
  import Editor from './lib/Editor.svelte'
  import SettingsDialog from './lib/SettingsDialog.svelte'
  import ConvertDialog from './lib/ConvertDialog.svelte'
  import ExportDialog from './lib/ExportDialog.svelte'
  import Modal from './lib/Modal.svelte'

  let screen = $state('home') // home | read | edit
  let csvPath = $state('')
  let mapping = $state(null)
  let detection = $state(null)
  let mappingPath = $state('')
  let targetPath = $state('')
  let saved = $state('') // JSON of the mapping as last saved or opened
  let raw = $state(null)
  let preview = $state(null)
  let issues = $state([])
  let dialog = $state(null)
  let message = $state(null)
  let confirmText = $state(null)
  let confirmResolve = null
  let version = $state('')

  // Compared with sorted keys: removing and re-adding an option moves it
  // in the object without changing the mapping.
  const canonical = (v) => JSON.stringify(v, (_, x) => (x && typeof x === 'object' && !Array.isArray(x)
    ? Object.fromEntries(Object.entries(x).filter(([, y]) => y !== undefined).sort(([a], [b]) => a.localeCompare(b))) : x))
  const dirty = $derived(mapping !== null && canonical(mapping) !== saved)
  const sources = $derived(sourcesOf(raw, mapping?.input?.header ?? true))
  const errorCount = $derived(issues.filter((i) => i.severity === 'error').length)
  const fileName = (p) => p.split(/[\\/]/).pop()

  let recent = $state([])

  api.version().then((v) => (version = v))
  const loadRecent = () => api.recent().then((r) => (recent = r ?? [])).catch(() => {})
  loadRecent()

  // Dropping a file on the window opens it: mappings by extension, anything
  // else as a CSV file.
  api.onFileDrop((paths) => {
    const path = paths?.[0]
    if (!path) return
    if (/\.ya?ml$/i.test(path)) openMapping(path)
    else openCSV(path)
  })

  function fail(e) { message = { kind: 'error', text: String(e?.message ?? e) } }
  function info(text) { message = { kind: 'ok', text } }
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

  // Raw preview while reading; output preview and checks while editing.
  $effect(() => {
    if (screen !== 'read' || !mapping || !csvPath) return
    const input = JSON.parse(JSON.stringify(mapping.input))
    const t = setTimeout(() => api.readRaw(csvPath, input).then((r) => (raw = r)).catch(fail), 200)
    return () => clearTimeout(t)
  })
  $effect(() => {
    if (screen !== 'edit' || !mapping) return
    // Read everything the preview depends on here, synchronously: Svelte
    // only re-runs an effect for what it reads before the first await or
    // timeout. (csvPath read inside the timeout was missed: opening a sample
    // file did not refresh the preview.)
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
      screen = 'read'
      message = null
      api.remember('csv', path).then(loadRecent)
    } catch (e) { fail(e) }
  }

  async function redetect() {
    try {
      const o = await api.open(csvPath)
      detection = o.detection
      mapping.input = o.mapping.input
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
      screen = 'edit'
      message = null
      api.remember('mapping', path).then(loadRecent)
      info(`Opened ${fileName(path)}. Open a sample CSV file to preview the conversion.`)
    } catch (e) { fail(e) }
  }

  async function openSample() {
    const path = await api.chooseFile('Sample CSV file for the preview', 'csv')
    if (!path) return
    try {
      raw = await api.readRaw(path, JSON.parse(JSON.stringify(mapping.input)))
      csvPath = path
      message = null
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
      api.remember('mapping', path).then(loadRecent)
      info(`Saved ${fileName(path)}${targetPath ? ` and ${fileName(targetPath)}` : ''}.`)
    } catch (e) { fail(e) }
  }

  async function home() {
    if (await leaveOk()) {
      screen = 'home'
      mapping = null
      saved = ''
    }
  }
</script>

<div class="app">
  <header class="bar">
    <button class="link brand" onclick={home}>lmmtfy <span>Studio</span></button>
    {#if mapping}
      <span class="title">
        <strong>{mapping.name}</strong>
        {#if dirty}<span class="badge warn" title="Unsaved changes">●</span>{/if}
        <span class="muted mono">{mappingPath ? fileName(mappingPath) : 'not saved yet'}</span>
      </span>
      {#if screen === 'edit'}
        <button class="link" onclick={() => (screen = 'read')} disabled={!csvPath}>← Reading settings</button>
        {#if csvPath}
          <span class="muted sample" title={csvPath}>sample: <span class="mono">{fileName(csvPath)}</span></span>
          <button class="link" onclick={openSample}>Change…</button>
        {:else}
          <button onclick={openSample}>Open sample CSV…</button>
        {/if}
      {/if}
      <span class="spacer"></span>
      {#if screen === 'edit'}
        <button onclick={() => (dialog = 'settings')}>Settings</button>
        <button onclick={() => (dialog = 'convert')} disabled={errorCount > 0}>Convert…</button>
        <button onclick={() => (dialog = 'export')} disabled={errorCount > 0}>Export converter…</button>
        <button class="primary" onclick={() => save()} disabled={errorCount > 0}>Save</button>
        <button onclick={() => save(true)} disabled={errorCount > 0}>Save as…</button>
      {/if}
    {:else}
      <span class="spacer"></span>
    {/if}
  </header>

  {#if message}
    <div class="message {message.kind}">
      <span>{message.text}</span>
      <button class="link" onclick={() => (message = null)}>Dismiss</button>
    </div>
  {/if}

  <main>
    {#if screen === 'home'}
      <div class="home">
        <h1>Let Me Map That For You</h1>
        <p class="muted">Describe once how a messy CSV file becomes the format your software expects. Then convert files here, from the command line, or with a standalone converter.</p>
        <div class="cards">
          <button class="card" onclick={() => openCSV()}><strong>New mapping from a CSV file</strong><span>Open a file, check how it is read, map its columns.</span></button>
          <button class="card" onclick={() => openMapping()}><strong>Open a mapping</strong><span>Edit a saved mapping, convert files, export converters.</span></button>
        </div>
        <p class="muted">or drop a CSV or mapping file on this window</p>
        {#if recent.length}
          <div class="recent panel">
            <h3>Recent</h3>
            {#each recent as r}
              <button class="link item" title={r.path} onclick={() => (r.kind === 'mapping' ? openMapping(r.path) : openCSV(r.path))}>
                <span class="badge">{r.kind === 'mapping' ? 'mapping' : 'CSV'}</span>
                <span class="name">{fileName(r.path)}</span>
                <span class="muted mono dir">{r.path.slice(0, -fileName(r.path).length)}</span>
              </button>
            {/each}
          </div>
        {/if}
        <p class="muted small">Version {version}</p>
      </div>
    {:else if screen === 'read' && mapping}
      <ReadStep bind:input={mapping.input} {raw} path={csvPath} notes={detection?.notes ?? []} onredetect={redetect} oncontinue={() => (screen = 'edit')} />
    {:else if screen === 'edit' && mapping}
      <Editor bind:mapping {sources} {detection} {preview} {issues} hasInput={!!csvPath} />
    {/if}
  </main>
</div>

{#if dialog === 'settings'}
  <SettingsDialog bind:mapping bind:targetPath onclose={() => (dialog = null)} onerror={fail} />
{:else if dialog === 'convert'}
  <ConvertDialog {mapping} input={csvPath} onclose={() => (dialog = null)} />
{:else if dialog === 'export'}
  <ExportDialog {mapping} onclose={() => (dialog = null)} />
{/if}

{#if confirmText}
  <Modal title="Please confirm" onclose={() => answer(false)}>
    <p>{confirmText}</p>
    {#snippet footer()}
      <button onclick={() => answer(false)}>Cancel</button>
      <button class="primary" onclick={() => answer(true)}>Discard changes</button>
    {/snippet}
  </Modal>
{/if}

<style>
  .app { display: flex; flex-direction: column; height: 100%; }
  .bar { display: flex; align-items: center; gap: 8px; padding: 8px 14px; border-bottom: 1px solid var(--line); background: var(--panel); flex-wrap: wrap; }
  .brand { font-weight: 700; font-size: 15px; color: var(--text); }
  .brand span { color: var(--accent); font-weight: 500; }
  .title { display: flex; gap: 8px; align-items: center; margin-left: 10px; }
  .sample { margin-left: 8px; }
  .message { display: flex; justify-content: space-between; align-items: center; padding: 6px 14px; }
  .message.error { background: var(--error-soft); color: var(--error); }
  .message.ok { background: var(--soft); color: var(--ok); }
  main { flex: 1; min-height: 0; padding: 12px; }
  .home { max-width: 720px; margin: 8vh auto 0; text-align: center; }
  .home h1 { font-size: 26px; margin-bottom: 6px; }
  .cards { display: grid; grid-template-columns: 1fr 1fr; gap: 14px; margin: 26px 0; }
  .card { display: flex; flex-direction: column; gap: 6px; text-align: left; padding: 18px; border-radius: 10px; }
  .card strong { font-size: 15px; }
  .card span { color: var(--muted); }
  .small { font-size: 11px; }
  .recent { text-align: left; padding: 6px 14px 10px; margin: 18px 0; }
  .item { display: flex; gap: 8px; align-items: center; width: 100%; padding: 5px 2px; color: var(--text); text-align: left; }
  .item .name { font-weight: 600; }
  .item .dir { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; direction: rtl; flex: 1; text-align: left; }
</style>
