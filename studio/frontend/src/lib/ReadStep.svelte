<script>
  // Step 1: how to read the file, with a live preview of the raw rows.
  import DataTable from './DataTable.svelte'
  import { list, joinList } from './mapping.js'

  let { input = $bindable(), raw, path, notes = [], onredetect, oncontinue } = $props()

  const encodings = ['utf-8', 'utf-16le', 'utf-16be', 'windows-1252', 'latin-1', 'iso-8859-15']
  const delimiters = [[',', 'comma ,'], [';', 'semicolon ;'], ['tab', 'tab'], ['|', 'pipe |'], ['space', 'space']]

  let nullText = $state(joinList(input.null_tokens))
  const width = $derived(raw?.width ?? 0)
  const rows = $derived((raw?.rows ?? []).map((r) => [...r.fields, ...Array(Math.max(0, width - r.fields.length)).fill('')]))
  const counts = $derived.by(() => {
    const c = { skipped: 0, header: 0, data: 0, footer: 0 }
    for (const r of raw?.rows ?? []) c[r.kind]++
    return c
  })
</script>

<div class="layout">
  <aside class="panel form">
    <h2>Read the file</h2>
    <p class="muted mono path" title={path}>{path}</p>

    <h3>Format</h3>
    <div class="fields">
      <label>Encoding
        <input list="encodings" bind:value={input.encoding} />
        <datalist id="encodings">{#each encodings as e}<option value={e}></option>{/each}</datalist>
      </label>
      <label>Delimiter
        <select bind:value={input.delimiter}>
          {#each delimiters as [v, l]}<option value={v}>{l}</option>{/each}
          {#if !delimiters.some(([v]) => v === input.delimiter)}<option value={input.delimiter}>{input.delimiter}</option>{/if}
        </select>
      </label>
      <label>Quote
        <select bind:value={input.quote}>
          <option value={'"'}>double quote "</option>
          <option value="'">single quote '</option>
          <option value="none">none</option>
        </select>
      </label>
      <label>Escape inside quotes
        <select bind:value={input.escape}>
          <option value="double">doubled ""</option>
          <option value="backslash">backslash \"</option>
          <option value="none">none</option>
        </select>
      </label>
    </div>

    <h3>Layout</h3>
    <div class="fields">
      <label class="inline"><input type="checkbox" bind:checked={input.header} /> First row is a header</label>
      <label>{input.header ? 'Header on line' : 'Data starts on line'}
        <input type="number" min="1" bind:value={input.header_row} />
      </label>
      {#if input.header}
        <label>Rows to skip after header <input type="number" min="0" bind:value={input.skip_rows_after_header} /></label>
      {/if}
      <label>Footer lines to skip <input type="number" min="0" bind:value={input.skip_footer} /></label>
      <label>Comment lines start with <input bind:value={input.comment_prefix} placeholder="e.g. #" /></label>
    </div>

    <h3>Values</h3>
    <div class="fields">
      <label class="inline"><input type="checkbox" bind:checked={input.trim} /> Trim spaces around values</label>
      <label>Read as empty
        <input bind:value={nullText} oninput={() => (input.null_tokens = list(nullText))} placeholder="NULL, N/A, -" />
      </label>
      <label>Rows with a different field count
        <select bind:value={input.ragged_rows}>
          <option value="error">are errors</option>
          <option value="pad">may be short (padded)</option>
          <option value="truncate">may be long (extra fields ignored)</option>
          <option value="lenient">may be short or long</option>
        </select>
      </label>
      <label>Duplicate header names
        <select bind:value={input.duplicate_headers}>
          <option value="error">are errors</option>
          <option value="suffix">are numbered (Name, Name_2)</option>
        </select>
      </label>
    </div>

    {#if notes.length}
      <h3>Detection notes</h3>
      {#each notes as n}<div class="issue warning">{n}</div>{/each}
    {/if}

    <div class="row actions">
      <button onclick={onredetect}>Detect again</button>
      <span class="spacer"></span>
      <button class="primary" onclick={oncontinue} disabled={!raw || raw.error || counts.data === 0}>Map columns →</button>
    </div>
  </aside>

  <section class="panel preview">
    <div class="row legend">
      <strong>Preview</strong>
      <span class="badge">{width} columns</span>
      <span class="muted">{counts.skipped} skipped · {counts.header} header · {counts.data} data · {counts.footer} footer (first rows)</span>
    </div>
    {#if raw?.error}<div class="issue error">{raw.error}</div>{/if}
    <div class="table">
      <DataTable {rows} lineNumbers={(raw?.rows ?? []).map((r) => r.line)} rowClass={(i) => raw.rows[i].kind} />
    </div>
  </section>
</div>

<style>
  .layout { display: grid; grid-template-columns: 330px 1fr; gap: 12px; height: 100%; min-height: 0; }
  .form { padding: 16px; overflow: auto; }
  .fields { display: grid; gap: 9px; }
  .path { word-break: break-all; margin: 0 0 6px; }
  .actions { margin-top: 18px; }
  .preview { display: flex; flex-direction: column; min-height: 0; min-width: 0; }
  .legend { padding: 10px 12px; border-bottom: 1px solid var(--line); }
  .table { flex: 1; min-height: 0; }
</style>
