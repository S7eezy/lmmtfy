<script>
  // A default value: fixed, or computed once per file from a column (its
  // first, lowest or highest value) or by a formula over aggregates.
  import FormulaField from './FormulaField.svelte'
  import * as M from './mapping.js'

  // value: the fixed default, written by onvalue (which knows when an empty
  // field means "no default"). placeholder: for the fixed value.
  let { r, mapping, path, col, sources = [], detectedFor = () => null, label = 'Default value', placeholder = '', onvalue } = $props()

  const KINDS = [['first', 'First value of'], ['lowest', 'Lowest / earliest of'], ['highest', 'Highest / latest of'], ['last', 'Last value of']]
  const parsed = $derived.by(() => {
    const m = (r.default_formula ?? '').match(/^\s*(first|last|lowest|highest)\(\s*\[((?:[^\]]|\]\])+)\]\s*\)\s*$/i)
    return m ? { kind: m[1].toLowerCase(), column: m[2].replaceAll(']]', ']').trim() } : null
  })
  let formulaMode = $state(false)
  $effect(() => { path; formulaMode = false })
  const mode = $derived(r.default_formula === undefined ? 'value' : parsed && !formulaMode ? parsed.kind : 'formula')

  const readable = $derived([
    ...M.columns(mapping).map((c) => ({ name: c.name, note: c.name === col.name ? 'this column' : '' })),
    ...Object.keys(mapping.helpers ?? {}).map((n) => ({ name: n, note: 'helper' })),
  ])
  const helperSources = $derived(new Set(Object.values(mapping.helpers ?? {}).map((h) => M.sourceKey(h))))
  const newSources = $derived(sources.filter((s) => !helperSources.has(s.key)))

  function setMode(m) {
    formulaMode = m === 'formula'
    if (m === 'value') { delete r.default_formula; return }
    delete r.default
    if (m === 'formula') { r.default_formula ??= ''; return }
    r.default_formula = `${m}([${(parsed?.column ?? col.name).replaceAll(']', ']]')}])`
  }
  function setColumn(v) {
    let name = v
    if (v.startsWith('source:')) {
      const s = sources.find((x) => x.key === v.slice(7))
      name = M.addHelper(mapping, s, detectedFor(s))
    }
    r.default_formula = `${parsed?.kind ?? 'first'}([${name.replaceAll(']', ']]')}])`
  }
</script>

<label class="default-field">{label}
  <div class="line">
    <select value={mode} onchange={(e) => setMode(e.target.value)}>
      <option value="value">Fixed value</option>
      {#each KINDS as [k, text]}<option value={k}>{text}…</option>{/each}
      <option value="formula">Formula</option>
    </select>
    {#if mode === 'value'}
      <input value={r.default ?? ''} oninput={(e) => onvalue(e.target.value)} {placeholder} />
    {:else if mode !== 'formula'}
      <select value={parsed.column} onchange={(e) => setColumn(e.target.value)}>
        {#each readable as c}<option value={c.name}>{c.name}{c.note ? ` (${c.note})` : ''}</option>{/each}
        {#if newSources.length}
          <optgroup label="Source columns (adds a helper)">
            {#each newSources as s}<option value="source:{s.key}">{s.label}</option>{/each}
          </optgroup>
        {/if}
      </select>
    {/if}
  </div>
</label>
{#if mode === 'formula'}
  <FormulaField bind:value={r.default_formula} {mapping} path="{path}.default_formula" self={col.name} colType={col.type} {sources} {detectedFor}
    placeholder="e.g. add_days(lowest([date]), -1)" />
{/if}
{#if mode !== 'value'}<p class="hint computed">Computed once per file, before rows are converted; empty values are left out.</p>{/if}

<style>
  .default-field { gap: 4px; }
  .line { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1.2fr); gap: 4px; }
  .line > * { width: 100%; min-width: 0; }
  .computed { margin: 2px 0 0; }
</style>
