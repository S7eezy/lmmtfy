<script>
  // Step 1: how to read the file, with a live preview of the raw rows.
  import { FileText, Rows3, TextCursorInput, Sparkles, RefreshCw, ArrowRight, FileSpreadsheet, CircleAlert } from '@lucide/svelte'
  import DataTable from './DataTable.svelte'
  import Section from './Section.svelte'
  import { list, joinList } from './mapping.js'

  let { input = $bindable(), raw, path, notes = [], onredetect, oncontinue, continueLabel = 'Next' } = $props()

  const encodings = ['utf-8', 'utf-16le', 'utf-16be', 'windows-1252', 'latin-1', 'iso-8859-15']
  const delimiters = [[',', ',', 'comma'], [';', ';', 'semicolon'], ['tab', 'tab', 'tab character'], ['|', '|', 'pipe']]
  const isStandard = $derived(delimiters.some(([v]) => v === input.delimiter) || input.delimiter === '\t')
  let otherDelimiter = $state(false)

  let nullText = $state(joinList(input.null_tokens))
  const width = $derived(raw?.width ?? 0)
  const rows = $derived((raw?.rows ?? []).map((r) => [...r.fields, ...Array(Math.max(0, width - r.fields.length)).fill('')]))
  const counts = $derived.by(() => {
    const c = { skipped: 0, header: 0, data: 0, footer: 0 }
    for (const r of raw?.rows ?? []) c[r.kind]++
    return c
  })
  const header = $derived(raw?.rows?.find((r) => r.kind === 'header'))
  const legend = [['header', 'header'], ['data', 'data'], ['skipped', 'skipped'], ['footer', 'footer']]
</script>

<div class="layout">
  <aside class="panel form">
    <div class="file">
      <FileSpreadsheet size={20} />
      <div>
        <strong title={path}>{path.split(/[\\/]/).pop()}</strong>
        <div class="muted mono small" title={path}>{path}</div>
      </div>
    </div>

    {#if notes.length}
      <div class="callout">
        <Sparkles size={15} />
        <div>
          <strong>Detected automatically</strong> — check the preview.
          {#each notes as n}<div class="note">{n}</div>{/each}
        </div>
      </div>
    {/if}

    <Section icon={FileText} title="Format">
      <div class="fields">
        <label>Encoding
          <input list="encodings" bind:value={input.encoding} />
          <datalist id="encodings">{#each encodings as e}<option value={e}></option>{/each}</datalist>
        </label>
        <label>Delimiter
          <span class="segmented">
            {#each delimiters as [v, l, title]}
              <button class:on={!otherDelimiter && (input.delimiter === v || (v === 'tab' && input.delimiter === '\t'))} {title}
                onclick={() => { otherDelimiter = false; input.delimiter = v }}>{l}</button>
            {/each}
            <button class:on={otherDelimiter || !isStandard} onclick={() => (otherDelimiter = true)}>other</button>
          </span>
          {#if otherDelimiter || !isStandard}
            <input class="mono short" maxlength="1" value={isStandard ? '' : input.delimiter} oninput={(e) => e.target.value && (input.delimiter = e.target.value)} placeholder="character" />
          {/if}
        </label>
        <div class="pair">
          <label>Quote
            <select bind:value={input.quote}>
              <option value={'"'}>double "</option>
              <option value="'">single '</option>
              <option value="none">none</option>
            </select>
          </label>
          <label>Escape in quotes
            <select bind:value={input.escape}>
              <option value="double">doubled ""</option>
              <option value="backslash">backslash \"</option>
              <option value="none">none</option>
            </select>
          </label>
        </div>
      </div>
    </Section>

    <Section icon={Rows3} title="Layout" hint="Which lines hold the header and the data.">
      <div class="fields">
        <label class="inline"><input type="checkbox" bind:checked={input.header} /> The file has a header row</label>
        <div class="pair">
          <label>{input.header ? 'Header on line' : 'Data starts on line'}
            <input type="number" min="1" bind:value={input.header_row} />
          </label>
          <label>Footer lines to skip <input type="number" min="0" value={input.skip_footer ?? 0} oninput={(e) => (input.skip_footer = Number(e.target.value) || 0)} /></label>
        </div>
        {#if input.header}
          <label>Rows to skip after the header <input type="number" min="0" value={input.skip_rows_after_header ?? 0} oninput={(e) => (input.skip_rows_after_header = Number(e.target.value) || 0)} /></label>
        {/if}
        <label>Ignore lines starting with <input bind:value={input.comment_prefix} placeholder="e.g. #" /></label>
      </div>
    </Section>

    <Section icon={TextCursorInput} title="Values">
      <div class="fields">
        <label class="inline"><input type="checkbox" bind:checked={input.trim} /> Trim spaces around values</label>
        <label>Read these values as empty
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
    </Section>

    <div class="row actions">
      <button onclick={onredetect} title="Detect the settings again from the file"><RefreshCw size={14} /> Detect again</button>
      <span class="spacer"></span>
      <button class="primary" onclick={oncontinue} disabled={!raw || raw.error || counts.data === 0}>{continueLabel} <ArrowRight size={14} /></button>
    </div>
  </aside>

  <section class="panel preview">
    <div class="row legend">
      <strong>Preview</strong>
      <span class="badge">{width} columns</span>
      {#each legend as [kind, label]}
        <span class="chip {kind}"><span class="dot"></span>{counts[kind]} {label}</span>
      {/each}
      <span class="spacer"></span>
      <span class="muted small">first rows of the file</span>
    </div>
    {#if raw?.error}<div class="issue error row"><CircleAlert size={14} /> {raw.error}</div>{/if}
    {#if counts.data > 0 && !header && input.header}
      <div class="issue warning">No header row found: check the header line.</div>
    {/if}
    <div class="table">
      <DataTable {rows} lineNumbers={(raw?.rows ?? []).map((r) => r.line)} rowClass={(i) => raw.rows[i].kind} />
    </div>
  </section>
</div>

<style>
  .layout { display: grid; grid-template-columns: 350px 1fr; gap: 12px; height: 100%; min-height: 0; }
  .form { padding: 14px 16px; overflow: auto; }
  .file { display: flex; gap: 10px; align-items: flex-start; color: var(--accent); margin-bottom: 4px; }
  .file strong { color: var(--text); word-break: break-all; }
  .small { font-size: 11px; word-break: break-all; }
  .callout { display: flex; gap: 8px; padding: 9px 11px; border-radius: 8px; background: var(--soft); color: var(--accent); margin: 10px 0 4px; font-size: 12px; }
  .callout strong { color: var(--text); }
  .note { color: var(--muted); margin-top: 3px; }
  .fields { display: grid; gap: 9px; }
  .pair { display: grid; grid-template-columns: 1fr 1fr; gap: 9px; }
  .short { width: 90px !important; margin-top: 4px; }
  .segmented { display: flex; }
  .segmented button { flex: 1; font-family: var(--mono); font-weight: 600; }
  .actions { margin-top: 14px; position: sticky; bottom: -14px; background: var(--panel); padding: 10px 0; }
  .actions button { display: inline-flex; gap: 6px; align-items: center; }
  .preview { display: flex; flex-direction: column; min-height: 0; min-width: 0; }
  .legend { padding: 10px 12px; border-bottom: 1px solid var(--line); }
  .chip { display: inline-flex; align-items: center; gap: 5px; font-size: 12px; color: var(--muted); }
  .dot { width: 9px; height: 9px; border-radius: 3px; }
  .chip.header .dot { background: var(--accent); }
  .chip.data .dot { background: var(--line); border: 1px solid var(--muted); }
  .chip.skipped .dot, .chip.footer .dot { background: repeating-linear-gradient(45deg, var(--muted) 0 2px, transparent 2px 4px); }
  .table { flex: 1; min-height: 0; }
</style>
