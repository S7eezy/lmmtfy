<script>
  // Options of the selected column, in reading order: what was detected in
  // the source, how the source is read, how the output is written. Empty
  // fields are removed, so defaults apply.
  import { Sparkles, FileInput, FileOutput, ShieldAlert, Replace, Plus, X, ArrowRight, Info, Sigma, Columns3, Trash2, Pin, Split, ListOrdered } from '@lucide/svelte'
  import CasesEditor from './CasesEditor.svelte'
  import DefaultField from './DefaultField.svelte'
  import Section from './Section.svelte'
  import ConfidenceBadge from './ConfidenceBadge.svelte'
  import TypeIcon from './TypeIcon.svelte'
  import TargetColumnForm from './TargetColumnForm.svelte'
  import FormulaField from './FormulaField.svelte'
  import * as M from './mapping.js'
  import { list, joinList, issuesFor, sourceKey } from './mapping.js'
  import { pct } from './confidence.js'

  // locked: the target is a linked file, its options are shown, not edited.
  // helper: col and r are a helper's ({ name, type } and the helper).
  let { col, r, mapping, issues = [], detected = null, locked = false, helper = false, sources = [], detectedFor = () => null,
    trace = null, onremove = () => {} } = $props()

  const hasSource = $derived(!!(r?.source || r?.source_index))
  const path = $derived(`${helper ? 'helpers' : 'columns'}.${col.name}`)
  const mine = $derived(issuesFor(issues, col.name, helper))
  // Shown by the formula field, which checks the formula as it is typed;
  // these are what only the whole mapping tells (circular references).
  const formulaIssues = $derived(mine.filter((i) => i.path.endsWith('.formula')))
  // Choosing "Source column" before choosing which one.
  let pendingSource = $state(false)
  $effect(() => { col; pendingSource = false })
  const mode = $derived(pendingSource ? 'source' : M.fillMode(r))

  const MODES = [['source', 'Source', FileInput], ['fixed', 'Fixed value', Pin], ['formula', 'Formula', Sigma],
    ['cases', 'Conditions', Split], ['sequence', 'Row number', ListOrdered]]
  // Helpers are not written: no fixed value (a formula can be one), no row numbers.
  const modes = $derived(helper ? MODES.filter(([m]) => m !== 'fixed' && m !== 'sequence') : MODES)
  function fill(m) {
    if (m === 'source') { pendingSource = !hasSource; return }
    pendingSource = false
    M.fillRule(r, m, helper ? { default: null } : col)
  }
  function setNumber(key, v) {
    if (v === '' || isNaN(Number(v))) delete r.sequence[key]
    else r.sequence[key] = Math.trunc(Number(v))
  }
  const fromLabel = { formula: 'formula', cases: 'conditions', sequence: 'row number', fixed: 'no source', source: 'no source' }
  function pick(key) {
    const s = sources.find((x) => x.key === key)
    if (!s) return
    if (helper) M.setHelperSource(mapping, col.name, s, detectedFor(s))
    else M.setSource(mapping, col.name, s, detectedFor(s))
    pendingSource = false
  }
  // A Unix timestamp is a number: read it through an integer helper.
  function epoch() {
    const s = sources.find((x) => x.key === sourceKey(r))
    if (!s) return
    const name = M.addHelper(mapping, s, detectedFor(s), 'integer')
    delete r.source
    delete r.source_index
    r.formula = `from_epoch([${name}], "ms")`
  }
  const temporal0 = $derived(col.type === 'date' || col.type === 'datetime')
  const examples = $derived(hasSource ? [
    { label: 'Convert units', formula: 'value / 1000', note: 'e.g. milliseconds to seconds' },
    { label: 'Round', formula: 'round(value, 2)' },
    { label: 'Change case', formula: 'upper(value)' },
    { label: 'Replace text', formula: 'replace(value, "old", "new")' },
    { label: 'Fill empty values from another column', formula: 'coalesce(value, [column])' },
    ...(temporal0 && !helper ? [{ label: 'Unix timestamp in milliseconds → datetime', note: 'Reads the source as a number, through a helper.', apply: epoch }] : []),
  ] : [
    { label: 'Join columns', formula: '[column] & " " & [other column]' },
    { label: 'If … else', formula: 'if([column] > 0, "yes", "no")' },
    { label: 'Several cases', formula: 'ifs([column] > 100, "high", [column] > 10, "medium", "low")' },
    { label: 'Compute', formula: 'round([price] * [quantity], 2)' },
    { label: 'Row number', formula: 'row()' },
    { label: 'Date of the conversion', formula: 'today()' },
  ])
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
    if (value === '' && (hasSource || r.formula !== undefined || col.default !== undefined)) delete r.default
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
      <span class="from" class:none={!hasSource}>{hasSource ? sourceKey(r) : fromLabel[mode]}</span>
      <ArrowRight size={14} />
      <span class="to"><TypeIcon type={col.type} size={15} /> {col.name}</span>
      {#if helper}<span class="helper-tag">helper · not written</span>
        <span class="spacer"></span><button class="icon delete" onclick={onremove} title="Remove the helper" aria-label="Remove the helper"><Trash2 size={14} /></button>{/if}
    </div>
  </header>

  {#each mine.filter((i) => !i.path.endsWith('.formula')) as i}<div class="issue {i.severity}">{i.path.replace(`${path}.`, '').replace(`target.columns.${col.name}.`, 'output ').replace(path, helper ? 'helper' : 'column')}: {i.message}</div>{/each}

  <Section icon={Columns3} title="Fill with">
    <div class="modes">
      {#each modes as [m, label, Icon]}
        <button class:on={mode === m} onclick={() => fill(m)}><Icon size={14} /> {label}</button>
      {/each}
    </div>
    {#if mode === 'source'}
      <label class="pick">Source column
        <select value={sourceKey(r)} onchange={(e) => pick(e.target.value)}>
          {#if !hasSource}<option value="" disabled>Choose a source column…</option>{/if}
          {#each sources as s}<option value={s.key}>{s.label}</option>{/each}
          {#if hasSource && !sources.some((s) => s.key === sourceKey(r))}<option value={sourceKey(r)}>{sourceKey(r)}{sources.length ? ' (not in this file)' : ''}</option>{/if}
        </select>
      </label>
      {#if hasSource}
        {#if r.formula === undefined}
          <button class="link add-formula" onclick={() => (r.formula = '')}><Plus size={13} /> Change the value with a formula</button>
        {:else}
          <div class="sub row"><span class="muted">Formula on <span class="mono">value</span>, the source value read as {col.type}:</span><span class="spacer"></span>
            <button class="link" onclick={() => delete r.formula}>Remove</button></div>
          <FormulaField bind:value={r.formula} {mapping} {path} self={col.name} hasValue colType={col.type} {sources} {detectedFor} {trace} {examples} problems={formulaIssues} placeholder="e.g. value / 1000" />
        {/if}
      {/if}
    {:else if mode === 'cases'}
      <CasesEditor {r} {mapping} {path} {col} {sources} {detectedFor} issues={mine} />
      {#if trace?.length}<p class="hint trace-hint">The first rows:</p>{/if}
      {#each trace ?? [] as t}
        <div class="traced row"><span class="muted">line {t.line}</span>{#each t.inputs ?? [] as i}<span class="mono muted">[{i.column}] {i.empty ? 'empty' : i.value}</span>{/each}<span class="spacer"></span><strong class:failed={t.error && !t.result}>{t.error && !t.result ? t.error : t.result || 'empty'}</strong></div>
      {/each}
    {:else if mode === 'sequence'}
      <div class="grid pick">
        <label>Start <input type="number" value={r.sequence.start ?? 1} oninput={(e) => setNumber('start', e.target.value)} /></label>
        <label>Step <input type="number" value={r.sequence.step ?? 1} oninput={(e) => setNumber('step', e.target.value)} /></label>
      </div>
      <p class="hint">Rows are numbered as they are written: a rejected row takes no number. Formulas cannot read these numbers; <span class="mono">row()</span> gives the input row number.</p>
    {:else if mode === 'fixed'}
      <div class="pick">
        <DefaultField {r} {mapping} {path} {col} {sources} {detectedFor} label="Value of every row" onvalue={setDefault}
          placeholder={col.default !== undefined ? `target default: ${col.default}` : 'empty'} />
      </div>
    {:else}
      <FormulaField bind:value={r.formula} {mapping} {path} self={col.name} colType={col.type} {sources} {detectedFor} {trace} {examples} problems={formulaIssues} placeholder={'e.g. [first name] & " " & [last name]'} />
    {/if}
  </Section>

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

  {#if helper}
    <Section icon={FileOutput} title="Type" hint="How formulas read the helper. It is written nowhere.">
      <div class="grid">
        <label>Type
          <select value={col.type} onchange={(e) => M.setHelperType(mapping, col.name, e.target.value, detected)}>
            {#each M.TYPES as t}<option value={t}>{t}</option>{/each}
          </select>
        </label>
      </div>
      <div class="default">
        <DefaultField {r} {mapping} {path} {col} {sources} {detectedFor} onvalue={(v) => (v === '' ? delete r.default : (r.default = v))}
          placeholder={{ decimal: '1234.5', integer: '12', date: '2024-01-31', datetime: '2024-01-31T10:00:00', time: '10:00:00', boolean: 'true' }[col.type] ?? 'none'} />
      </div>
    </Section>
  {:else}
    <Section icon={FileOutput} title="Writing the output" hint={locked ? 'Set by the target file: change it with Edit target.' : 'How the value is checked and written in the output file.'}>
      <TargetColumnForm {col} readonly={locked} />
      {#if mode !== 'fixed' && mode !== 'sequence'}
        <div class="default">
          <DefaultField {r} {mapping} {path} {col} {sources} {detectedFor} onvalue={setDefault}
            placeholder={col.default !== undefined ? `target default: ${col.default}` : 'none'} />
          {#if mode === 'formula' || mode === 'cases'}<p class="hint override">Used when {mode === 'formula' ? 'the formula fails' : 'a condition fails'} and invalid values “use the default”.</p>{/if}
        </div>
      {/if}
      {#if r.default === '' && col.default !== undefined}
        <p class="hint override">Empty, which overrides the target default. <button class="link" onclick={() => delete r.default}>Use “{col.default}”</button></p>
      {/if}
    </Section>
  {/if}

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
  .flow .spacer { flex: 1; }
  .helper-tag { font-size: 11px; color: var(--muted); background: var(--neutral-soft); border-radius: 9px; padding: 1px 8px; }
  .delete { color: var(--muted); }
  .delete:hover { color: var(--error); }
  .pick { margin-top: 10px; }
  .modes { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 4px; }
  .modes button { display: inline-flex; gap: 5px; align-items: center; justify-content: center; padding: 5px 6px; font-size: 12px; white-space: nowrap; overflow: hidden; }
  .modes button :global(svg) { flex-shrink: 0; }
  .modes button.on { background: var(--accent); border-color: var(--accent); color: var(--accent-text); font-weight: 600; }
  .trace-hint { margin: 8px 0 2px; }
  .traced { font-size: 12px; gap: 6px; padding: 2px 4px; border-top: 1px solid var(--line); flex-wrap: nowrap; overflow: hidden; }
  .traced .mono { white-space: nowrap; }
  .traced .failed { color: var(--error); font-weight: 500; }
  .add-formula { display: inline-flex; gap: 4px; align-items: center; margin-top: 6px; padding-left: 0; }
  .sub { margin: 10px 0 4px; font-size: 12px; flex-wrap: nowrap; }
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
