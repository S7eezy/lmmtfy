<script>
  // How a target column is checked and written: its options in the target.
  // Editable in the target editor and for targets defined with the mapping;
  // with a linked target file (readonly), a summary of what the file says.
  import { joinList, list } from './mapping.js'

  // withDefault shows the target's own default (the mapping editor has its
  // own default field, which wins).
  let { col, readonly = false, withDefault = false } = $props()

  const temporal = $derived(col.type === 'date' || col.type === 'datetime' || col.type === 'time')
  const numeric = $derived(col.type === 'integer' || col.type === 'decimal')
  const formats = { date: 'YYYY-MM-DD', datetime: 'YYYY-MM-DDTHH:mm:ss', time: 'HH:mm:ss' }

  function set(key, value, number = false) {
    if (value === '' || value === null || value === undefined || value === false) delete col[key]
    else col[key] = number ? Number(value) : value
  }
  function setList(key, text) {
    const v = list(text)
    if (v.length) col[key] = v
    else delete col[key]
  }

  // The options the target sets, in words.
  const summary = $derived.by(() => {
    const s = [[ 'Type', col.type ]]
    s.push(['Required', col.required ? 'yes: no empty values' : 'no'])
    if (col.default !== undefined) s.push(['Target default', col.default === '' ? 'empty' : col.default])
    if (temporal) s.push(['Format', col.format || formats[col.type]])
    if (col.type === 'datetime' && col.timezone) s.push(['Time zone', col.timezone])
    if (col.type === 'decimal') {
      s.push(['Decimals', col.scale ?? 'as read'])
      if (col.rounding) s.push(['Rounding', col.rounding])
      s.push(['Decimal separator', col.decimal_sep || '.'])
    }
    if (numeric && col.thousands_sep) s.push(['Thousands separator', `“${col.thousands_sep}”`])
    if (numeric && col.min != null) s.push(['Minimum', col.min])
    if (numeric && col.max != null) s.push(['Maximum', col.max])
    if (col.type === 'boolean') s.push(['Written as', `${col.true_value ?? 'true'} / ${col.false_value ?? 'false'}`])
    if (col.case && col.case !== 'none') s.push(['Case', col.case])
    if (col.max_length) s.push(['Max length', `${col.max_length}${col.on_overflow === 'truncate' ? ', longer values truncated' : ''}`])
    if (col.pattern) s.push(['Pattern', col.pattern])
    if (col.allowed_values?.length) s.push(['Allowed values', joinList(col.allowed_values)])
    return s
  })
</script>

{#if readonly}
  <dl class="summary">
    {#each summary as [k, v]}<dt>{k}</dt><dd class:mono={k === 'Format' || k === 'Pattern'}>{v}</dd>{/each}
  </dl>
{:else}
  <div class="grid">
    <label class="inline"><input type="checkbox" checked={!!col.required} onchange={(e) => set('required', e.target.checked)} /> Required (no empty values)</label>
    {#if withDefault}
      <label>Default value
        <input value={col.default ?? ''} oninput={(e) => set('default', e.target.value)} placeholder="none" />
      </label>
    {/if}
    {#if col.type === 'string'}
      <label>Case
        <select value={col.case ?? ''} onchange={(e) => set('case', e.target.value)}>
          <option value="">unchanged</option><option value="upper">UPPER</option><option value="lower">lower</option>
        </select>
      </label>
      <label>Max length <input type="number" min="1" value={col.max_length ?? ''} oninput={(e) => set('max_length', e.target.value, true)} placeholder="no limit" /></label>
      {#if col.max_length}
        <label>When longer
          <select value={col.on_overflow ?? ''} onchange={(e) => set('on_overflow', e.target.value)}>
            <option value="">error</option><option value="truncate">truncate</option>
          </select>
        </label>
      {/if}
      <label>Pattern (whole value) <input class="mono" value={col.pattern ?? ''} oninput={(e) => set('pattern', e.target.value)} placeholder="[A-Z]{'{'}2{'}'}" /></label>
      <label class="wide">Allowed values <input value={joinList(col.allowed_values)} oninput={(e) => setList('allowed_values', e.target.value)} placeholder="A, B, C" /></label>
    {:else if numeric}
      {#if col.type === 'decimal'}
        <label>Decimals <input type="number" min="0" max="30" value={col.scale ?? ''} oninput={(e) => set('scale', e.target.value, true)} placeholder="as read" /></label>
        <label>Rounding
          <select value={col.rounding ?? ''} onchange={(e) => set('rounding', e.target.value)}>
            <option value="">half up (2.345 → 2.35)</option><option value="half_even">half even (2.345 → 2.34)</option><option value="truncate">truncate</option>
          </select>
        </label>
        <label>Decimal separator <input value={col.decimal_sep ?? ''} oninput={(e) => set('decimal_sep', e.target.value)} placeholder="." /></label>
      {/if}
      <label>Thousands separator <input value={col.thousands_sep ?? ''} oninput={(e) => set('thousands_sep', e.target.value)} placeholder="none" /></label>
      <label>Minimum <input value={col.min ?? ''} oninput={(e) => set('min', e.target.value)} /></label>
      <label>Maximum <input value={col.max ?? ''} oninput={(e) => set('max', e.target.value)} /></label>
    {:else if col.type === 'boolean'}
      <label>Write true as <input value={col.true_value ?? ''} oninput={(e) => set('true_value', e.target.value)} placeholder="true" /></label>
      <label>Write false as <input value={col.false_value ?? ''} oninput={(e) => set('false_value', e.target.value)} placeholder="false" /></label>
    {:else}
      <label>Format <input class="mono" value={col.format ?? ''} oninput={(e) => set('format', e.target.value)} placeholder={formats[col.type]} /></label>
      {#if col.type === 'datetime'}
        <label>Time zone <input value={col.timezone ?? ''} oninput={(e) => set('timezone', e.target.value)} placeholder="UTC" /></label>
      {/if}
    {/if}
  </div>
{/if}

<style>
  .grid { grid-template-columns: 1fr 1fr; }
  .wide { grid-column: 1 / -1; }
  .summary { display: grid; grid-template-columns: auto 1fr; gap: 4px 12px; margin: 0; font-size: 12px; }
  .summary dt { color: var(--muted); }
  .summary dd { margin: 0; word-break: break-word; }
</style>
