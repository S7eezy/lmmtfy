<script>
  // Options of the selected column, in reading order: what was detected in
  // the source, how the source is read, how the output is written. Empty
  // fields are removed, so defaults apply.
  import { Sparkles, FileInput, FileOutput, ShieldAlert, Replace, Plus, X, ArrowRight, Info } from '@lucide/svelte'
  import Section from './Section.svelte'
  import ConfidenceBadge from './ConfidenceBadge.svelte'
  import TypeIcon from './TypeIcon.svelte'
  import TargetColumnForm from './TargetColumnForm.svelte'
  import { list, joinList, issuesFor, sourceKey } from './mapping.js'
  import { pct } from './confidence.js'

  // locked: the target is a linked file, its options are shown, not edited.
  let { col, r, issues = [], detected = null, locked = false } = $props()

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
  // An empty field means no default of the mapping's own: the target's
  // default applies, if any. Only a column with neither a source nor a
  // target default needs an explicit empty default.
  function setDefault(value) {
    if (value === '' && (hasSource || col.default !== undefined)) delete r.default
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

  <Section icon={FileOutput} title="Writing the output" hint={locked ? 'Set by the target file: change it with Edit target.' : 'How the value is checked and written in the output file.'}>
    <TargetColumnForm {col} readonly={locked} />
    <div class="grid default">
      <label>Default value {hasSource ? '' : '(every row)'}
        <input value={r.default ?? ''} oninput={(e) => setDefault(e.target.value)} placeholder={col.default !== undefined ? `target default: ${col.default}` : hasSource ? 'none' : 'empty'} />
      </label>
      {#if r.default === '' && col.default !== undefined}
        <p class="hint override">Empty, which overrides the target default. <button class="link" onclick={() => delete r.default}>Use “{col.default}”</button></p>
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
  .default { margin-top: 10px; }
  .override { grid-column: 1 / -1; margin: 0; }
  .override button { padding: 0; }
</style>
