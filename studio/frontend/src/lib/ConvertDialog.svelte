<script>
  // Convert a whole file with the mapping being edited. The file's header
  // is checked first: missing columns (with the column each one may have
  // become), columns found under another name, columns not used.
  import { Play, FileInput, FileOutput, CircleCheck, CircleX, Link, Info, TriangleAlert } from '@lucide/svelte'
  import Modal from './Modal.svelte'
  import * as api from './api.js'
  import * as M from './mapping.js'

  let { mapping = $bindable(), input = '', onclose } = $props()

  let source = $state(input)
  let output = $state('')
  let running = $state(false)
  let result = $state(null)
  let error = $state('')

  // The header check, whenever the file or the mapping changes.
  let check = $state(null)
  let checkError = $state('')
  $effect(() => {
    const path = source
    const snapshot = JSON.parse(JSON.stringify(mapping))
    if (!path) { check = null; checkError = ''; return }
    const t = setTimeout(() => api.checkInput(path, snapshot).then((c) => { check = c; checkError = '' }).catch((e) => { check = null; checkError = String(e?.message ?? e) }), 200)
    return () => clearTimeout(t)
  })
  const missing = $derived((check?.columns ?? []).filter((c) => c.status === 'missing'))
  const aliased = $derived((check?.columns ?? []).filter((c) => c.alias))
  function accept(c) {
    const r = M.ruleOf(mapping, c.target)
    if (r) M.addAlias(r, c.suggestion)
  }

  async function pickInput() { source = (await api.chooseFile('File to convert', 'csv')) || source }
  async function pickOutput() {
    const base = source ? source.replace(/\.[^./\\]+$/, '') + '.converted.csv' : 'output.csv'
    output = (await api.chooseSaveFile('Write the converted file to', base, 'csv')) || output
  }
  async function run() {
    running = true
    error = ''
    result = null
    try { result = await api.convert(mapping, source, output) } catch (e) { error = String(e) }
    running = false
  }
</script>

<Modal title="Convert a file" icon={Play} {onclose}>
  <div class="fields">
    <label><span class="lbl"><FileInput size={13} /> Input file</span>
      <div class="row"><input class="mono grow" bind:value={source} /><button onclick={pickInput}>Choose…</button></div>
    </label>
    <label><span class="lbl"><FileOutput size={13} /> Output file</span>
      <div class="row"><input class="mono grow" bind:value={output} /><button onclick={pickOutput}>Choose…</button></div>
    </label>
  </div>
  {#if checkError}<div class="issue error">{checkError}</div>{/if}
  {#if check}
    <div class="check">
      {#each missing as c}
        <div class="issue {c.action === 'error' ? 'error' : 'warning'} drift">
          {#if c.action === 'error'}<CircleX size={14} />{:else}<TriangleAlert size={14} />{/if}
          <span><span class="mono">{c.source}</span> (for {c.target}) is not in this file{c.action === 'error' ? '' : `: ${{ empty: 'left empty', default: 'default used', omitted: 'column omitted' }[c.action]}`}.{#if c.suggestion} Renamed to <span class="mono">{c.suggestion}</span>?{/if}</span>
          {#if c.suggestion}<span class="spacer"></span><button onclick={() => accept(c)} title="Adds “{c.suggestion}” as another name of the source, in the mapping"><Link size={12} /> Also accept “{c.suggestion}”</button>{/if}
        </div>
      {/each}
      {#each aliased as c}
        <div class="issue info drift"><Info size={14} /><span><span class="mono">{c.source}</span> is found as <span class="mono">{c.alias}</span> (another name).</span></div>
      {/each}
      {#if check.unused?.length}
        <div class="issue info drift"><Info size={14} /><span>Not used: <span class="mono">{check.unused.join(', ')}</span></span></div>
      {/if}
      {#if check.ok && !missing.length}<div class="issue ok drift"><CircleCheck size={14} /> Every source column is in this file.</div>{/if}
    </div>
  {/if}
  {#if error}<div class="issue error">{error}</div>{/if}
  {#if result}
    {#each (result.issues ?? []).filter((i) => i.severity === 'error') as i}<div class="issue error">{i.path}: {i.message}</div>{/each}
    {#if result.failure}
      <div class="issue error result"><CircleX size={16} /> Conversion failed, no output written: {result.failure}</div>
    {:else if result.summary}
      <div class="issue ok result"><CircleCheck size={16} /> Done: {result.summary.rows_read} rows read, {result.summary.rows_written} written{result.summary.rows_filtered ? `, ${result.summary.rows_filtered} filtered` : ''}{result.summary.rows_duplicate ? `, ${result.summary.rows_duplicate} duplicate` : ''}, {result.summary.rows_rejected} rejected in {result.summary.duration_seconds}s.</div>
      {#if result.rejects}<p>Rejected rows: <span class="mono">{result.rejects}</span></p>{/if}
    {/if}
    {#each (result.events ?? []).slice(0, 50) as e}
      <div class="issue {e.level === 'error' ? 'error' : 'warning'}">{[e.line && `line ${e.line}`, e.column].filter(Boolean).join(' · ')}{e.line || e.column ? ': ' : ''}{e.message}</div>
    {/each}
    {#if (result.events ?? []).length > 50}<p class="muted">… {result.events.length - 50} more messages</p>{/if}
  {/if}
  {#snippet footer()}
    <button onclick={onclose}>Close</button>
    <button class="primary" onclick={run} disabled={running || !source || !output || (check && !check.ok)} title={check && !check.ok ? 'Required source columns are missing from this file' : ''}>{running ? 'Converting…' : 'Convert'}</button>
  {/snippet}
</Modal>

<style>
  .fields { display: grid; gap: 10px; margin-bottom: 10px; }
  .grow { flex: 1; }
  .lbl { display: inline-flex; gap: 5px; align-items: center; }
  .result { display: flex; gap: 8px; align-items: center; font-size: 13px; padding: 8px 10px; }
  .issue.ok { background: var(--soft); color: var(--ok); }
  .check { display: grid; gap: 4px; margin-bottom: 10px; }
  .drift { display: flex; gap: 7px; align-items: center; font-size: 12px; }
  .drift button { display: inline-flex; gap: 4px; align-items: center; font-size: 12px; padding: 2px 9px; white-space: nowrap; }
  .drift :global(svg) { flex-shrink: 0; }
</style>
