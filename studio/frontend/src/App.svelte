<script>
  // lmmtfy Studio. Two kinds of files: mappings and targets.
  //
  // A new mapping goes through four steps: read the file, choose its target,
  // name it, map the columns. An opened mapping goes straight to the columns;
  // every step stays reachable from the stepper. The target editor edits a
  // target file on its own, from the home screen or from a mapping linked to
  // it (Edit target).
  //
  // The mapping being edited lives here, its target inline; children edit it
  // through bindings, and previews are refreshed shortly after each change.
  // targetMode says where the target lives: "file" (linked, locked), "new"
  // (written to its own file on the next save) or "inline".
  import { FilePlus2, Settings2, Play, Package, Save, FileCog, Upload, Target, ArrowLeft, Sheet, SquareDashed } from '@lucide/svelte'
  import * as api from './lib/api.js'
  import * as M from './lib/mapping.js'
  import { toast } from './lib/toast.svelte.js'
  import ReadStep from './lib/ReadStep.svelte'
  import TargetStep from './lib/TargetStep.svelte'
  import SetupStep from './lib/SetupStep.svelte'
  import Editor from './lib/Editor.svelte'
  import TargetEditor from './lib/TargetEditor.svelte'
  import Stepper from './lib/Stepper.svelte'
  import Toasts from './lib/Toasts.svelte'
  import SettingsDialog from './lib/SettingsDialog.svelte'
  import TargetSettingsDialog from './lib/TargetSettingsDialog.svelte'
  import ConvertDialog from './lib/ConvertDialog.svelte'
  import ExportDialog from './lib/ExportDialog.svelte'
  import Modal from './lib/Modal.svelte'
  import Home from './lib/Home.svelte'

  const STEPS = ['read', 'target', 'setup', 'edit']
  let screen = $state('home') // home | read | target | setup | edit | tedit
  let reached = $state(0) // furthest step of the mapping, in STEPS
  let csvPath = $state('')
  let mapping = $state(null)
  let detection = $state(null)
  let mappingPath = $state('')
  let targetMode = $state('new')
  let targetPath = $state('')
  // The target this mapping defines (drafted from the file, or a copy of the
  // current target), kept while trying target files in the Target step.
  // Not reactive: only the Target step reads it, when it opens.
  const draft = { target: null, columns: {} }
  let draftIsCopy = $state(false)
  let pendingTarget = $state('') // target to link when the Target step opens
  let saved = $state('') // the mapping as last saved or opened
  let raw = $state(null)
  let preview = $state(null)
  let traced = $state('') // the column whose formula the preview traces
  let aggregateAll = $state(false) // aggregates over the whole sample file
  let issues = $state([])
  let dialog = $state(null)
  let confirmBox = $state(null) // { text, title, yes }
  let confirmResolve = null
  let version = $state('')
  let recent = $state([])
  let dropping = $state(false)
  // Columns a linked target file mapped by name: the editor lists them for
  // review. reviewKey changes to bring the Review list to the front.
  let autoMatched = $state([])
  let reviewKey = $state(0)

  // The target editor: { id, target, path, saved, detection, from, dir,
  // renames }. from is "home" or "mapping"; renames are the columns renamed
  // as of the last save, which the mapping follows when going back to it.
  let tgt = $state(null)
  let tgtIssues = $state([])
  let targetEditor = $state(null)
  let tgtCount = 0

  // Compared with sorted keys: removing and re-adding an option moves it
  // in the object without changing the mapping.
  const canonical = (v) => JSON.stringify(v, (_, x) => (x && typeof x === 'object' && !Array.isArray(x)
    ? Object.fromEntries(Object.entries(x).filter(([, y]) => y !== undefined).sort(([a], [b]) => a.localeCompare(b))) : x))
  const mappingState = () => canonical({ mapping, targetMode, targetPath })
  const mappingDirty = $derived(mapping !== null && mappingState() !== saved)
  const targetDirty = $derived(tgt !== null && canonical(tgt.target) !== tgt.saved)
  const dirty = $derived(mappingDirty || targetDirty)
  const sources = $derived(M.sources(raw, mapping?.input?.header ?? true))
  const errorCount = $derived(issues.filter((i) => i.severity === 'error').length)
  const tgtErrors = $derived(tgtIssues.filter((i) => i.severity === 'error').length)
  const fileName = (p) => p.split(/[\\/]/).pop()
  const dirOf = (p) => p.replace(/[^\\/]+$/, '')
  const slug = (s) => M.targetName(s || 'target').replaceAll('_', '-')
  const recentTargets = $derived(recent.filter((r) => r.kind === 'target'))

  api.version().then((v) => (version = v))
  const loadRecent = () => api.recent().then((r) => (recent = r ?? [])).catch(() => {})
  loadRecent()

  // The window asks before closing with unsaved changes (see app.go).
  $effect(() => { api.setDirty(dirty) })

  // Dropping a file on the window opens it.
  api.onFileDrop((paths) => {
    dropping = false
    const path = paths?.[0]
    if (path) openFile(path)
  })

  const fail = (e) => toast('error', String(e?.message ?? e))
  function ask(text, { title = 'Unsaved changes', yes = 'Discard changes' } = {}) {
    confirmBox = { text, title, yes }
    return new Promise((resolve) => (confirmResolve = resolve))
  }
  function answer(yes) {
    confirmBox = null
    confirmResolve?.(yes)
  }
  async function leaveOk() {
    return !dirty || (await ask(mappingDirty ? 'The mapping has unsaved changes. Discard them?' : 'The target has unsaved changes. Discard them?'))
  }

  function keydown(e) {
    if (!((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 's')) return
    if (screen === 'edit') {
      e.preventDefault()
      if (errorCount === 0) save(e.shiftKey)
      else toast('error', 'Fix the errors before saving.')
    } else if (screen === 'tedit') {
      e.preventDefault()
      if (tgtErrors === 0) saveTarget(e.shiftKey)
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
    const column = traced
    const all = aggregateAll
    const t = setTimeout(async () => {
      try {
        if (path) {
          const res = await api.preview(path, snapshot, { trace: column, aggregate_all: all })
          res.traced = column
          preview = res
          issues = res.issues ?? []
        } else {
          issues = await api.check(snapshot)
        }
      } catch (e) { fail(e) }
    }, 300)
    return () => clearTimeout(t)
  })
  $effect(() => {
    if (screen !== 'tedit' || !tgt) return
    const snapshot = JSON.parse(JSON.stringify(tgt.target))
    const t = setTimeout(() => api.checkTarget(snapshot).then((r) => (tgtIssues = r ?? [])).catch(fail), 250)
    return () => clearTimeout(t)
  })

  function go(step) {
    screen = step
    reached = Math.max(reached, STEPS.indexOf(step))
  }
  function closeAll() {
    mapping = null
    saved = ''
    tgt = null
  }
  function targetLinked({ matched }) {
    autoMatched = matched
    reviewKey++
    loadRecent()
  }

  // Mappings ---------------------------------------------------------------

  // openCSV starts a new mapping from a CSV file; with target, that target
  // file is chosen in the Target step.
  async function openCSV(path = '', target = '') {
    if (!(await leaveOk())) return
    path ||= await api.chooseFile('Open a CSV file', 'csv')
    if (!path) return
    try {
      const o = await api.open(path)
      closeAll()
      csvPath = path
      detection = o.detection
      mapping = o.mapping
      mapping.target.inline.name ||= o.mapping.name
      Object.assign(draft, { target: M.clone(mapping.target.inline), columns: M.clone(mapping.columns ?? {}) })
      draftIsCopy = false
      mappingPath = ''
      targetMode = 'new'
      targetPath = ''
      pendingTarget = target
      raw = null
      preview = null
      autoMatched = []
      reached = 0
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
    let l
    try {
      l = await api.loadMapping(path)
    } catch (e) {
      // Not a mapping: maybe a target file without the usual name.
      try {
        await api.loadTarget(path)
      } catch {
        fail(e)
        return
      }
      return openTarget(path, true)
    }
    closeAll()
    mapping = l.mapping
    mappingPath = path
    targetPath = l.target_path ?? ''
    targetMode = targetPath ? 'file' : 'inline'
    Object.assign(draft, { target: M.clone(mapping.target.inline), columns: M.clone(mapping.columns ?? {}) })
    draftIsCopy = targetMode === 'file'
    saved = mappingState()
    detection = null
    preview = null
    raw = null
    csvPath = ''
    autoMatched = []
    reached = STEPS.length - 1
    screen = 'edit'
    api.remember('mapping', path).then(loadRecent)
    toast('info', `Opened ${fileName(path)}. Open a sample CSV file to preview conversions.`)
  }

  function openFile(path) {
    if (/\.target\.ya?ml$/i.test(path)) openTarget(path)
    else if (/\.ya?ml$/i.test(path)) openMapping(path)
    else openCSV(path)
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
      const base = mappingPath ? dirOf(mappingPath) : csvPath ? dirOf(csvPath) : ''
      path = await api.chooseSaveFile('Save the mapping', base + slug(mapping.name || 'mapping') + '.mapping.yaml', 'mapping')
      if (!path) return
    }
    const writeTarget = targetMode === 'new'
    let tp = targetMode === 'inline' ? '' : targetPath
    if (writeTarget && !tp) {
      // A location chosen in the Target step was confirmed by the save
      // dialog; the default one, next to the mapping, is checked here.
      tp = dirOf(path) + slug(mapping.target.inline.name || mapping.name) + '.target.yaml'
      if (await api.exists(tp) && !(await ask(`${fileName(tp)} already exists next to the mapping. Replace it with this mapping's new target?`, { title: 'Replace a target file', yes: 'Replace it' }))) return
    }
    try {
      await api.saveMapping(mapping, path, tp, writeTarget)
      mappingPath = path
      if (writeTarget) {
        targetMode = 'file'
        targetPath = tp
        draftIsCopy = true
        api.remember('target', tp)
      }
      saved = mappingState()
      reached = STEPS.length - 1
      api.remember('mapping', path).then(loadRecent)
      toast('ok', writeTarget
        ? `Saved ${fileName(path)} and its new target ${fileName(tp)}. The target is now a file of its own: change it with Edit target.`
        : `Saved ${fileName(path)}.`)
    } catch (e) { fail(e) }
  }

  // Targets ----------------------------------------------------------------

  function startTarget(t, { path = '', detection = null, from = 'home', dir = '' } = {}) {
    tgt = { id: ++tgtCount, target: t, path, saved: path ? canonical(t) : '', detection, from, dir, renames: {} }
    tgtIssues = []
    screen = 'tedit'
  }

  async function openTarget(path = '', checked = false) {
    if (!checked && !(await leaveOk())) return
    path ||= await api.chooseFile('Open a target', 'target')
    if (!path) return
    try {
      const t = await api.loadTarget(path)
      closeAll()
      startTarget(t, { path })
      api.remember('target', path).then(loadRecent)
    } catch (e) { fail(e) }
  }

  async function newTarget(fromExample) {
    dialog = null
    if (!(await leaveOk())) return
    if (!fromExample) {
      closeAll()
      startTarget(M.blankTarget())
      return
    }
    const path = await api.chooseFile('An example of the expected file', 'csv')
    if (!path) return
    try {
      const d = await api.draftTarget(path)
      d.target.name = slug(fileName(path).replace(/\.[^.]+$/, ''))
      closeAll()
      startTarget(d.target, { detection: d.detection, dir: dirOf(path) })
      toast('info', `Drafted ${d.target.columns.length} columns from ${fileName(path)}: check names, types and formats.`)
    } catch (e) { fail(e) }
  }

  async function saveTarget(as = false) {
    let path = tgt.path
    // From a mapping, the target is the file the mapping links to.
    if (!path || (as && tgt.from !== 'mapping')) {
      path = await api.chooseSaveFile('Save the target', (tgt.dir || dirOf(tgt.path)) + slug(tgt.target.name) + '.target.yaml', 'target')
      if (!path) return
    }
    try {
      await api.saveTarget(tgt.target, path)
      tgt.path = path
      tgt.saved = canonical(tgt.target)
      tgt.renames = targetEditor?.renames() ?? {}
      api.remember('target', path).then(loadRecent)
      toast('ok', `Saved ${fileName(path)}.`)
    } catch (e) { fail(e) }
  }

  // Edit target: the target editor on the file the mapping links to.
  async function editTarget() {
    dialog = null
    try {
      startTarget(await api.loadTarget(targetPath), { path: targetPath, from: 'mapping' })
    } catch (e) { fail(e) }
  }

  // Leaving the target editor. Back in a mapping, its target is reloaded
  // and its rules follow renamed columns.
  async function closeTarget() {
    if (targetDirty && !(await ask('The target has unsaved changes. Discard them?'))) return
    if (tgt.from !== 'mapping') {
      tgt = null
      screen = 'home'
      return
    }
    const renames = tgt.renames
    tgt = null
    screen = 'edit'
    try {
      const before = canonical(mapping.target.inline)
      const t = await api.loadTarget(targetPath)
      mapping.target.inline = t
      M.renameRules(mapping, renames)
      autoMatched = autoMatched.map((n) => renames[n] ?? n)
      if (canonical(t) !== before) {
        reviewKey++
        toast('info', 'Target reloaded: the Review list shows what needs a decision.')
      }
    } catch (e) { fail(e) }
  }

  async function home() {
    if (await leaveOk()) {
      closeAll()
      screen = 'home'
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
    {#if screen === 'tedit' && tgt}
      <span class="doc-title">
        <Target size={15} />
        <strong>{tgt.target.name || 'untitled target'}</strong>
        {#if targetDirty}<span class="unsaved" title="Unsaved changes">●</span>{/if}
        {#if tgt.path}<span class="mono path" title={tgt.path}>{fileName(tgt.path)}</span>{/if}
      </span>
      <span class="spacer"></span>
      {#if tgt.from === 'mapping'}
        <button onclick={closeTarget} title="Back to the mapping, with this target reloaded"><ArrowLeft size={14} /> Back to {mapping?.name || 'the mapping'}</button>
      {:else if tgt.path && !targetDirty}
        <button onclick={() => openCSV('', tgt.path)} title="Map a CSV file onto this target"><FilePlus2 size={14} /> New mapping for this target</button>
      {/if}
      <button onclick={() => (dialog = 'tsettings')} title="Name and file format"><Settings2 size={14} /> Settings</button>
      <button class="primary" onclick={() => saveTarget()} disabled={tgtErrors > 0} title="Save (Ctrl/⌘ S, with Shift: save as)"><Save size={14} /> Save</button>
    {:else if mapping}
      <span class="doc-title">
        <FileCog size={15} />
        <strong>{mapping.name || 'untitled'}</strong>
        {#if mappingDirty}<span class="unsaved" title="Unsaved changes">●</span>{/if}
      </span>
      <span class="divider"></span>
      <Stepper current={screen} enabled={STEPS.map((s, i) => i <= reached && (s !== 'read' || !!csvPath))} onstep={goto} />
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
      <Home {recent} {version} onnewmapping={() => openCSV()} onopenmapping={openMapping} onopencsv={openCSV}
        onnewtarget={() => (dialog = 'newtarget')} onopentarget={openTarget} />
    {:else if screen === 'read' && mapping}
      <ReadStep bind:input={mapping.input} {raw} path={csvPath} notes={detection?.notes ?? []} onredetect={redetect}
        continueLabel={reached >= 3 ? 'Back to the columns' : 'Next: target'} oncontinue={() => go(reached >= 3 ? 'edit' : 'target')} />
    {:else if screen === 'target' && mapping}
      <TargetStep bind:mapping bind:mode={targetMode} bind:targetPath {draft} copy={draftIsCopy} {sources} {detection} recent={recentTargets}
        preset={pendingTarget} onpreset={() => (pendingTarget = '')} onlinked={targetLinked}
        onback={csvPath ? () => (screen = 'read') : null} backLabel="Reading settings"
        continueLabel={reached >= 3 ? 'Back to the columns' : 'Next: settings'} oncontinue={() => go(reached >= 3 ? 'edit' : 'setup')} />
    {:else if screen === 'setup' && mapping}
      <SetupStep bind:mapping onback={() => (screen = 'target')} oncontinue={() => go('edit')} />
    {:else if screen === 'edit' && mapping}
      <Editor bind:mapping bind:traced bind:aggregateAll {sources} {detection} {preview} {issues} {autoMatched} {reviewKey} samplePath={csvPath} onsample={openSample}
        locked={targetMode === 'file'} targetFile={targetPath} onedittarget={editTarget} />
    {:else if screen === 'tedit' && tgt}
      {#key tgt.id}
        <TargetEditor bind:this={targetEditor} bind:target={tgt.target} path={tgt.path} detection={tgt.detection} issues={tgtIssues} />
      {/key}
    {/if}
  </main>
</div>

{#if dropping}
  <div class="dropzone"><Upload size={36} /><span>Drop a CSV file, a mapping or a target to open it</span></div>
{/if}

{#if dialog === 'settings'}
  <SettingsDialog bind:mapping {targetMode} {targetPath} onclose={() => (dialog = null)}
    ontargetstep={() => { dialog = null; go('target') }} onedittarget={editTarget} />
{:else if dialog === 'tsettings' && tgt}
  <TargetSettingsDialog bind:target={tgt.target} onclose={() => (dialog = null)} />
{:else if dialog === 'newtarget'}
  <Modal title="New target" icon={Target} onclose={() => (dialog = null)}>
    <p class="muted">Where should its columns come from?</p>
    <div class="starts">
      <button class="choice" onclick={() => newTarget(true)}>
        <Sheet size={20} />
        <span><span class="title">From an example of the expected file…</span>
          <span class="desc">A file the receiving software exports or documents. Its columns, types and file format are drafted for you.</span></span>
      </button>
      <button class="choice" onclick={() => newTarget(false)}>
        <SquareDashed size={20} />
        <span><span class="title">Blank</span><span class="desc">Start with one column and add the others.</span></span>
      </button>
    </div>
  </Modal>
{:else if dialog === 'convert'}
  <ConvertDialog {mapping} input={csvPath} onclose={() => (dialog = null)} />
{:else if dialog === 'export'}
  <ExportDialog {mapping} onclose={() => (dialog = null)} />
{/if}

{#if confirmBox}
  <Modal title={confirmBox.title} onclose={() => answer(false)}>
    <p>{confirmBox.text}</p>
    {#snippet footer()}
      <button onclick={() => answer(false)}>Cancel</button>
      <button class="primary" onclick={() => answer(true)}>{confirmBox.yes}</button>
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
  .doc-title { display: inline-flex; gap: 6px; align-items: center; margin-left: 6px; color: var(--muted); }
  .doc-title strong { color: var(--text); max-width: 220px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .doc-title .path { margin-left: 4px; }
  .unsaved { color: var(--warn); font-size: 12px; }
  .divider { width: 1px; height: 20px; background: var(--line); margin: 0 4px; }
  main { flex: 1; min-height: 0; padding: 12px; }
  .starts { display: grid; gap: 8px; margin: 8px 0 4px; }
  .starts :global(svg) { color: var(--accent); flex-shrink: 0; margin-top: 2px; }
  .dropzone { position: fixed; inset: 10px; border: 3px dashed var(--accent); border-radius: 16px; background: color-mix(in srgb, var(--bg) 88%, transparent); display: flex; flex-direction: column; gap: 12px; align-items: center; justify-content: center; color: var(--accent); font-size: 16px; font-weight: 600; z-index: 25; pointer-events: none; }
</style>
