<script>
  // Convert a whole file with the mapping being edited.
  import { Play, FileInput, FileOutput, CircleCheck, CircleX } from '@lucide/svelte'
  import Modal from './Modal.svelte'
  import * as api from './api.js'

  let { mapping, input = '', onclose } = $props()

  let source = $state(input)
  let output = $state('')
  let running = $state(false)
  let result = $state(null)
  let error = $state('')

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
  {#if error}<div class="issue error">{error}</div>{/if}
  {#if result}
    {#each (result.issues ?? []).filter((i) => i.severity === 'error') as i}<div class="issue error">{i.path}: {i.message}</div>{/each}
    {#if result.failure}
      <div class="issue error result"><CircleX size={16} /> Conversion failed, no output written: {result.failure}</div>
    {:else if result.summary}
      <div class="issue ok result"><CircleCheck size={16} /> Done: {result.summary.rows_read} rows read, {result.summary.rows_written} written, {result.summary.rows_rejected} rejected in {result.summary.duration_seconds}s.</div>
      {#if result.rejects}<p>Rejected rows: <span class="mono">{result.rejects}</span></p>{/if}
    {/if}
    {#each (result.events ?? []).slice(0, 50) as e}
      <div class="issue {e.level === 'error' ? 'error' : 'warning'}">{[e.line && `line ${e.line}`, e.column].filter(Boolean).join(' · ')}{e.line || e.column ? ': ' : ''}{e.message}</div>
    {/each}
    {#if (result.events ?? []).length > 50}<p class="muted">… {result.events.length - 50} more messages</p>{/if}
  {/if}
  {#snippet footer()}
    <button onclick={onclose}>Close</button>
    <button class="primary" onclick={run} disabled={running || !source || !output}>{running ? 'Converting…' : 'Convert'}</button>
  {/snippet}
</Modal>

<style>
  .fields { display: grid; gap: 10px; margin-bottom: 10px; }
  .grow { flex: 1; }
  .lbl { display: inline-flex; gap: 5px; align-items: center; }
  .result { display: flex; gap: 8px; align-items: center; font-size: 13px; padding: 8px 10px; }
  .issue.ok { background: var(--soft); color: var(--ok); }
</style>
