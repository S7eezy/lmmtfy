<script>
  // Options of the selected column, in reading order: what was detected in
  // the source, how the source is read, how the output is written. Empty
  // fields are removed, so defaults apply.
  import { Sparkles, FileInput, FileOutput, ShieldAlert, Replace, Plus, X, ArrowRight, Info } from '@lucide/svelte'
  import Section from './Section.svelte'
  import ConfidenceBadge from './ConfidenceBadge.svelte'
  import TypeIcon from './TypeIcon.svelte'
  import { list, joinList, issuesFor, sourceKey } from './mapping.js'
  import { pct } from './confidence.js'

  let { col, r, issues = [], detected = null } = $props()

  const hasSource = $derived(!!(r?.source || r?.source_index))
  const mine = $derived(issuesFor(issues, col.name))
  const temporal = $derived(col.type === 'date' || col.type === 'datetime' || col.type === 'time')
  const numeric = $derived(col.type === 'integer' || col.type === 'decimal')

  function set(obj, key, value, number = false) {
    if (value === '' || value === null || value === undefined) delete obj[key]
    else obj[key] = number ? Number(value) : value
  }
  function setList(obj, key, text) {
    const v = list(text)
    if (v.length) obj[key] = v
    else delete obj[key]
  }
  function setDefault(value) {
    if (value === '' && hasSource) delete r.default
    else r.default = value
  }
  function addPair() { r.map = [...(r.map ?? []), { from: '', to: '' }] }
  function removePair(i) {
    r.map.splice(i, 1)
    if (!r.map.length) { delete r.map; r.map_unmatched = 'keep' }
  }
</script>

<div class="details">
  <header class="title">
    <div class="flow">
      <span class="from" class:none={!hasSource}>{hasSource ? sourceKey(r) : 'no source'}</span>
      <ArrowRight size={14} />
      <span class="to"><TypeIcon type={col.type} size={15} /> {col.name}</span>
    </div>
  </header>

  {#each mine as i}<div class="issue {i.severity}">{i.path.replace(`columns.${col.name}.`, '').replace(`target.columns.${col.name}.`, 'output ')}: {i.message}</div>{/each}

  {#if detected}
    <Section icon={Sparkles} title="Detected in the source">
      <div class="row"><ConfidenceBadge {detected} /><span class="muted small">{detected.values} values{detected.empty ? `, ${detected.empty} empty` : ''}</span></div>
      <ConfidenceBadge {detected} bar />
      {#if detected.examples?.length}
        <div class="examples">{#each detected.examples as e}<span class="mono example">{e}</span>{/each}</div>
      {/if}
      {#if detected.failing?.length}<div class="issue warning">Rejected by {detected.type}: <span class="mono">{detected.failing.join(' · ')}</span></div>{/if}
      {#each detected.notes ?? [] as n}<div class="issue warning">{n}</div>{/each}
      {#if detected.type === 'string' && detected.near_type && detected.near_type !== col.type}
        <p class="hint with-icon"><Info size={12} /> {pct(detected.near_confidence)} of values read as {detected.near_type}: fix the source data, or keep text.</p>
      {/if}
    </Section>
  {/if}

  {#if hasSource}
    <Section icon={FileInput} title="Reading the source" hint="How values of the source column are understood.">
      <div class="grid">
        <label>Missing source column
          <select value={r.if_missing} onchange={(e) => { r.if_missing = e.target.value; r.optional = e.target.value !== 'error' || undefined }}>
            <option value="error">required: error</option>
            <option value="empty">optional: empty</option>
            <option value="default">optional: default</option>
            <option value="omit">optional: omit column</option>
          </select>
        </label>
        <label>Empty value
          <select bind:value={r.if_empty}>
            <option value="keep">leave empty</option>
            <option value="default">use the default</option>
            <option value="error">error</option>
          </select>
        </label>
        <label>Trim spaces
          <select value={r.trim === undefined ? '' : String(r.trim)} onchange={(e) => (e.target.value === '' ? delete r.trim : (r.trim = e.target.value === 'true'))}>
            <option value="">as reading settings</option>
            <option value="true">yes</option>
            <option value="false">no</option>
          </select>
        </label>
        {#if col.type === 'decimal'}
          <label>Decimal separator <input value={r.decimal_sep ?? ''} oninput={(e) => set(r, 'decimal_sep', e.target.value)} placeholder="." /></label>
        {/if}
        {#if numeric}
          <label>Thousands separator <input value={r.thousands_sep ?? ''} oninput={(e) => set(r, 'thousands_sep', e.target.value)} placeholder="none" /></label>
        {/if}
        {#if col.type === 'boolean'}
          <label>Values meaning true <input value={joinList(r.true_values)} oninput={(e) => setList(r, 'true_values', e.target.value)} placeholder="true, t, yes, y, 1" /></label>
          <label>Values meaning false <input value={joinList(r.false_values)} oninput={(e) => setList(r, 'false_values', e.target.value)} placeholder="false, f, no, n, 0" /></label>
        {/if}
        {#if temporal}
          <label class="wide">Formats, tried in order <input class="mono" value={joinList(r.formats)} oninput={(e) => setList(r, 'formats', e.target.value)} placeholder="e.g. DD/MM/YYYY — empty: the output format" /></label>
          {#if col.type === 'datetime'}
            <label>Time zone of values <input value={r.timezone ?? ''} oninput={(e) => set(r, 'timezone', e.target.value)} placeholder="as the output" /></label>
          {/if}
        {/if}
      </div>
      {#if temporal}<p class="tokens hint">Tokens: <span class="mono">YYYY YY MM M DD D HH hh h mm ss .SSS A Z</span></p>{/if}
      {#if r.if_missing === 'omit'}
        <div class="issue warning">Omitting changes the output columns when the source column is missing: the target software may reject such files.</div>
      {/if}
    </Section>

    <Section icon={Replace} title="Value map" hint={r.map?.length ? 'Source values replaced before conversion.' : 'Replace source values before conversion, e.g. codes by labels.'}>
      {#each r.map ?? [] as pair, i}
        <div class="pair">
          <input class="mono" bind:value={pair.from} placeholder="source value" />
          <ArrowRight size={14} />
          <input class="mono" bind:value={pair.to} placeholder="replaced by" />
          <button class="icon" onclick={() => removePair(i)} aria-label="Remove"><X size={13} /></button>
        </div>
      {/each}
      <div class="row">
        <button onclick={addPair}><Plus size={13} /> {r.map?.length ? 'Add a value' : 'Add a value map'}</button>
        {#if r.map?.length}
          <label class="inline">Other values
            <select bind:value={r.map_unmatched}>
              <option value="keep">kept</option><option value="error">are errors</option><option value="default">get the default</option>
            </select>
          </label>
        {/if}
      </div>
    </Section>
  {/if}

  <Section icon={FileOutput} title="Writing the output" hint="How the value is checked and written in the output file.">
    <div class="grid">
      <label class="inline"><input type="checkbox" checked={!!col.required} onchange={(e) => set(col, 'required', e.target.checked || '')} /> Required (no empty values)</label>
      <label>Default value {hasSource ? '' : '(every row)'}
        <input value={r.default ?? ''} oninput={(e) => setDefault(e.target.value)} placeholder={col.default ?? (hasSource ? 'none' : 'empty')} />
      </label>
      {#if col.type === 'string'}
        <label>Case
          <select value={col.case ?? ''} onchange={(e) => set(col, 'case', e.target.value)}>
            <option value="">unchanged</option><option value="upper">UPPER</option><option value="lower">lower</option>
          </select>
        </label>
        <label>Max length <input type="number" min="1" value={col.max_length ?? ''} oninput={(e) => set(col, 'max_length', e.target.value, true)} placeholder="no limit" /></label>
        {#if col.max_length}
          <label>When longer
            <select value={col.on_overflow ?? ''} onchange={(e) => set(col, 'on_overflow', e.target.value)}>
              <option value="">error</option><option value="truncate">truncate</option>
            </select>
          </label>
        {/if}
        <label>Pattern (whole value) <input class="mono" value={col.pattern ?? ''} oninput={(e) => set(col, 'pattern', e.target.value)} placeholder="[A-Z]{'{'}2{'}'}" /></label>
        <label class="wide">Allowed values <input value={joinList(col.allowed_values)} oninput={(e) => setList(col, 'allowed_values', e.target.value)} placeholder="A, B, C" /></label>
      {:else if numeric}
        {#if col.type === 'decimal'}
          <label>Decimals <input type="number" min="0" max="30" value={col.scale ?? ''} oninput={(e) => set(col, 'scale', e.target.value, true)} placeholder="as read" /></label>
          <label>Rounding
            <select value={col.rounding ?? ''} onchange={(e) => set(col, 'rounding', e.target.value)}>
              <option value="">half up (2.345 → 2.35)</option><option value="half_even">half even (2.345 → 2.34)</option><option value="truncate">truncate</option>
            </select>
          </label>
          <label>Decimal separator <input value={col.decimal_sep ?? ''} oninput={(e) => set(col, 'decimal_sep', e.target.value)} placeholder="." /></label>
        {/if}
        <label>Thousands separator <input value={col.thousands_sep ?? ''} oninput={(e) => set(col, 'thousands_sep', e.target.value)} placeholder="none" /></label>
        <label>Minimum <input value={col.min ?? ''} oninput={(e) => set(col, 'min', e.target.value)} /></label>
        <label>Maximum <input value={col.max ?? ''} oninput={(e) => set(col, 'max', e.target.value)} /></label>
      {:else if col.type === 'boolean'}
        <label>Write true as <input value={col.true_value ?? ''} oninput={(e) => set(col, 'true_value', e.target.value)} placeholder="true" /></label>
        <label>Write false as <input value={col.false_value ?? ''} oninput={(e) => set(col, 'false_value', e.target.value)} placeholder="false" /></label>
      {:else}
        <label>Format <input class="mono" value={col.format ?? ''} oninput={(e) => set(col, 'format', e.target.value)} placeholder={{ date: 'YYYY-MM-DD', datetime: 'YYYY-MM-DDTHH:mm:ss', time: 'HH:mm:ss' }[col.type]} /></label>
        {#if col.type === 'datetime'}
          <label>Time zone <input value={col.timezone ?? ''} oninput={(e) => set(col, 'timezone', e.target.value)} placeholder="UTC" /></label>
        {/if}
      {/if}
    </div>
  </Section>

  <Section icon={ShieldAlert} title="Invalid values">
    <label>For this column
      <select value={r.on_error ?? ''} onchange={(e) => set(r, 'on_error', e.target.value)}>
        <option value="">as mapping settings</option>
        <option value="fail_file">fail the file</option>
        <option value="skip_row">reject the row</option>
        <option value="default">use the default</option>
      </select>
    </label>
  </Section>
</div>

<style>
  .details { padding: 12px 16px 20px; }
  .title { padding-bottom: 8px; border-bottom: 1px solid var(--line); }
  .flow { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; color: var(--muted); }
  .from { font-family: var(--mono); font-size: 12px; background: var(--soft); color: var(--accent); padding: 2px 8px; border-radius: 6px; }
  .from.none { background: var(--neutral-soft); color: var(--muted); font-style: italic; font-family: inherit; }
  .to { display: inline-flex; gap: 5px; align-items: center; font-weight: 700; font-size: 15px; color: var(--text); word-break: break-all; }
  .grid { grid-template-columns: 1fr 1fr; }
  .wide { grid-column: 1 / -1; }
  .small { font-size: 11px; }
  .examples { display: flex; flex-wrap: wrap; gap: 4px; margin-top: 8px; }
  .example { background: var(--neutral-soft); padding: 1px 6px; border-radius: 5px; max-width: 100%; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .pair { display: flex; gap: 6px; align-items: center; margin-bottom: 5px; color: var(--muted); }
  .pair input { flex: 1; }
  .row button { display: inline-flex; gap: 4px; align-items: center; }
  .hint.with-icon { display: flex; gap: 4px; align-items: center; }
  .tokens .mono { word-spacing: 4px; }
</style>
