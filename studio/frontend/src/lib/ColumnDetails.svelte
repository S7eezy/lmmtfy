<script>
  // Options of the selected column: how it is read (rule) and written
  // (target column). Empty fields are removed, so defaults apply.
  import { list, joinList, issuesFor } from './mapping.js'

  let { col, r, issues = [], detected = null } = $props()

  const hasSource = $derived(!!(r?.source || r?.source_index))
  const mine = $derived(issuesFor(issues, col.name))

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
  <h2>{col.name} <span class="badge">{col.type}</span></h2>
  {#if detected}
    <p class="muted">Detected: {detected.type}, {Math.round(detected.confidence * 100)}% of {detected.values} values{#if detected.examples?.length}, e.g. <span class="mono">{detected.examples.slice(0, 3).join(' · ')}</span>{/if}</p>
    {#each detected.notes ?? [] as n}<div class="issue warning">{n}</div>{/each}
  {/if}
  {#each mine as i}<div class="issue {i.severity}">{i.path.replace(`columns.${col.name}.`, '').replace(`target.columns.${col.name}.`, 'target ')}: {i.message}</div>{/each}

  <h3>Values</h3>
  <div class="grid">
    <label class="inline"><input type="checkbox" checked={!!col.required} onchange={(e) => set(col, 'required', e.target.checked || '')} /> Required</label>
    <label>Default {hasSource ? '' : '(every row)'}
      <input value={r.default ?? ''} oninput={(e) => setDefault(e.target.value)} placeholder={col.default ?? (hasSource ? 'none' : 'empty')} />
    </label>
    {#if hasSource}
      <label>When empty
        <select bind:value={r.if_empty}>
          <option value="keep">leave empty</option>
          <option value="default">use the default</option>
          <option value="error">error</option>
        </select>
      </label>
      <label>When the source column is missing
        <select value={r.if_missing} onchange={(e) => { r.if_missing = e.target.value; r.optional = e.target.value !== 'error' || undefined }}>
          <option value="error">error (required column)</option>
          <option value="empty">optional: leave empty</option>
          <option value="default">optional: use the default</option>
          <option value="omit">optional: omit from output</option>
        </select>
      </label>
    {/if}
    <label>On error
      <select value={r.on_error ?? ''} onchange={(e) => set(r, 'on_error', e.target.value)}>
        <option value="">as the whole mapping</option>
        <option value="fail_file">fail the file</option>
        <option value="skip_row">reject the row</option>
        <option value="default">use the default</option>
      </select>
    </label>
    {#if hasSource}
      <label>Trim spaces
        <select value={r.trim === undefined ? '' : String(r.trim)} onchange={(e) => (e.target.value === '' ? delete r.trim : (r.trim = e.target.value === 'true'))}>
          <option value="">as the input settings</option>
          <option value="true">yes</option>
          <option value="false">no</option>
        </select>
      </label>
    {/if}
  </div>
  {#if r.if_missing === 'omit'}
    <div class="issue warning">Omitting changes the output columns when the source column is missing: the target software may reject such files.</div>
  {/if}

  <h3>Type options</h3>
  <div class="grid">
    {#if col.type === 'string'}
      <label>Case
        <select value={col.case ?? ''} onchange={(e) => set(col, 'case', e.target.value)}>
          <option value="">unchanged</option><option value="upper">UPPER</option><option value="lower">lower</option>
        </select>
      </label>
      <label>Max length <input type="number" min="1" value={col.max_length ?? ''} oninput={(e) => set(col, 'max_length', e.target.value, true)} /></label>
      {#if col.max_length}
        <label>When longer
          <select value={col.on_overflow ?? ''} onchange={(e) => set(col, 'on_overflow', e.target.value)}>
            <option value="">error</option><option value="truncate">truncate</option>
          </select>
        </label>
      {/if}
      <label>Pattern (whole value) <input class="mono" value={col.pattern ?? ''} oninput={(e) => set(col, 'pattern', e.target.value)} placeholder="[A-Z]{'{'}2{'}'}" /></label>
      <label>Allowed values <input value={joinList(col.allowed_values)} oninput={(e) => setList(col, 'allowed_values', e.target.value)} placeholder="A, B, C" /></label>
    {:else if col.type === 'integer' || col.type === 'decimal'}
      {#if col.type === 'decimal'}
        <label>Read: decimal separator <input value={r.decimal_sep ?? ''} oninput={(e) => set(r, 'decimal_sep', e.target.value)} placeholder="." /></label>
      {/if}
      <label>Read: thousands separator <input value={r.thousands_sep ?? ''} oninput={(e) => set(r, 'thousands_sep', e.target.value)} placeholder="none" /></label>
      {#if col.type === 'decimal'}
        <label>Write: decimal separator <input value={col.decimal_sep ?? ''} oninput={(e) => set(col, 'decimal_sep', e.target.value)} placeholder="." /></label>
      {/if}
      <label>Write: thousands separator <input value={col.thousands_sep ?? ''} oninput={(e) => set(col, 'thousands_sep', e.target.value)} placeholder="none" /></label>
      {#if col.type === 'decimal'}
        <label>Decimals (scale) <input type="number" min="0" max="30" value={col.scale ?? ''} oninput={(e) => set(col, 'scale', e.target.value, true)} placeholder="as read" /></label>
        <label>Rounding
          <select value={col.rounding ?? ''} onchange={(e) => set(col, 'rounding', e.target.value)}>
            <option value="">half up (2.345 → 2.35)</option><option value="half_even">half even (2.345 → 2.34)</option><option value="truncate">truncate</option>
          </select>
        </label>
      {/if}
      <label>Minimum <input value={col.min ?? ''} oninput={(e) => set(col, 'min', e.target.value)} /></label>
      <label>Maximum <input value={col.max ?? ''} oninput={(e) => set(col, 'max', e.target.value)} /></label>
    {:else if col.type === 'boolean'}
      <label>Read as true <input value={joinList(r.true_values)} oninput={(e) => setList(r, 'true_values', e.target.value)} placeholder="true, t, yes, y, 1" /></label>
      <label>Read as false <input value={joinList(r.false_values)} oninput={(e) => setList(r, 'false_values', e.target.value)} placeholder="false, f, no, n, 0" /></label>
      <label>Write true as <input value={col.true_value ?? ''} oninput={(e) => set(col, 'true_value', e.target.value)} placeholder="true" /></label>
      <label>Write false as <input value={col.false_value ?? ''} oninput={(e) => set(col, 'false_value', e.target.value)} placeholder="false" /></label>
    {:else}
      <label>Read formats <input class="mono" value={joinList(r.formats)} oninput={(e) => setList(r, 'formats', e.target.value)} placeholder="DD/MM/YYYY, DD-MM-YYYY" /></label>
      <label>Write format <input class="mono" value={col.format ?? ''} oninput={(e) => set(col, 'format', e.target.value)} placeholder={{ date: 'YYYY-MM-DD', datetime: 'YYYY-MM-DDTHH:mm:ss', time: 'HH:mm:ss' }[col.type]} /></label>
      {#if col.type === 'datetime'}
        <label>Read time zone <input value={r.timezone ?? ''} oninput={(e) => set(r, 'timezone', e.target.value)} placeholder="as written" /></label>
        <label>Write time zone <input value={col.timezone ?? ''} oninput={(e) => set(col, 'timezone', e.target.value)} placeholder="UTC" /></label>
      {/if}
    {/if}
  </div>
  {#if col.type === 'date' || col.type === 'datetime' || col.type === 'time'}
    <p class="muted small">Tokens: YYYY YY MM M DD D HH hh h mm ss .SSS A Z — e.g. DD/MM/YYYY HH:mm</p>
  {/if}

  {#if hasSource}
    <h3>Value map</h3>
    {#each r.map ?? [] as pair, i}
      <div class="row pair">
        <input class="mono" bind:value={pair.from} placeholder="source value" />
        <span>→</span>
        <input class="mono" bind:value={pair.to} placeholder="replaced by" />
        <button class="icon" onclick={() => removePair(i)} aria-label="Remove">✕</button>
      </div>
    {/each}
    <div class="row">
      <button onclick={addPair}>+ Add value</button>
      {#if r.map?.length}
        <label class="inline">Other values
          <select bind:value={r.map_unmatched}>
            <option value="keep">kept</option><option value="error">are errors</option><option value="default">get the default</option>
          </select>
        </label>
      {/if}
    </div>
  {/if}
</div>

<style>
  .details { padding: 14px 16px; }
  .details h2 { display: flex; gap: 8px; align-items: center; word-break: break-all; }
  .grid { grid-template-columns: 1fr 1fr; }
  .pair { margin-bottom: 5px; flex-wrap: nowrap; }
  .pair input { flex: 1; }
  .small { font-size: 11px; }
</style>
