<script>
  // Conditions filling a column: the first true one gives the value.
  // Simple conditions ([column] > value, empty tests) get a builder; any
  // other is edited as formula text. Values are written like defaults, or
  // computed by a formula.
  import { Plus, X, ArrowUp, ArrowDown, Sigma, TextCursorInput } from '@lucide/svelte'
  import FormulaField from './FormulaField.svelte'
  import * as M from './mapping.js'

  let { r, mapping, path, col, sources = [], detectedFor = () => null, issues = [] } = $props()

  const OPS = [['=', 'is'], ['<>', 'is not'], ['<', '<'], ['<=', '≤'], ['>', '>'], ['>=', '≥'], ['empty', 'is empty'], ['filled', 'is not empty']]

  // Cases shown as formula text, by index (and for those the builder cannot show).
  let asText = $state(new Set())
  const readable = $derived([
    ...M.columns(mapping).filter((c) => c.name !== col.name).map((c) => ({ name: c.name, type: c.type, note: '' })),
    ...Object.entries(mapping.helpers ?? {}).map(([n, h]) => ({ name: n, type: h.type, note: 'helper' })),
  ])
  const helperSources = $derived(new Set(Object.values(mapping.helpers ?? {}).map((h) => M.sourceKey(h))))
  const newSources = $derived(sources.filter((s) => !helperSources.has(s.key)))

  function setPart(i, part, v) {
    const k = r.cases[i]
    const cond = { ...(M.parseCondition(k.when) ?? { column: '', op: '=', value: '' }), [part]: v }
    if (part === 'column' && v.startsWith('source:')) {
      const s = sources.find((x) => x.key === v.slice(7))
      cond.column = M.addHelper(mapping, s, detectedFor(s))
    }
    k.when = M.buildCondition(cond, M.typeOf(mapping, cond.column))
  }
  function add() { r.cases = [...r.cases, { when: '', then: '' }] }
  function remove(i) {
    r.cases.splice(i, 1)
    if (!r.cases.length) r.cases = [{ when: '', then: '' }]
  }
  function move(i, d) {
    const [k] = r.cases.splice(i, 1)
    r.cases.splice(i + d, 0, k)
  }
  function thenFormula(k, on) {
    if (on) { k.then_formula = k.then ?? ''; delete k.then }
    else { k.then = ''; delete k.then_formula }
  }
  function elseMode(mode) {
    delete r.else
    delete r.else_formula
    if (mode === 'value') r.else = ''
    if (mode === 'formula') r.else_formula = ''
  }
  const elseMode_ = $derived(r.else_formula !== undefined ? 'formula' : r.else !== undefined ? 'value' : 'none')
  const caseIssues = (i) => issues.filter((x) => x.path.startsWith(`${path}.cases[${i}]`))
  const placeholder = { integer: '0', decimal: '0.00', boolean: 'true', date: '2024-01-31', datetime: '2024-01-31T10:00:00', time: '10:00:00' }
</script>

<div class="cases">
  {#each r.cases as k, i}
    {@const cond = M.parseCondition(k.when)}
    {@const text = asText.has(i) || !cond}
    <div class="case">
      <div class="head row">
        <span class="word">{i === 0 ? 'When' : 'Else when'}</span>
        <span class="spacer"></span>
        <button class="icon" title={text ? 'Edit with the builder' : 'Edit as formula'} disabled={text && !cond}
          onclick={() => { const s = new Set(asText); s.has(i) ? s.delete(i) : s.add(i); asText = s }}>
          {#if text}<TextCursorInput size={13} />{:else}<Sigma size={13} />{/if}</button>
        <button class="icon" title="Move up" disabled={i === 0} onclick={() => move(i, -1)}><ArrowUp size={13} /></button>
        <button class="icon" title="Move down" disabled={i === r.cases.length - 1} onclick={() => move(i, 1)}><ArrowDown size={13} /></button>
        <button class="icon" title="Remove the case" onclick={() => remove(i)}><X size={13} /></button>
      </div>
      {#if text}
        <FormulaField bind:value={k.when} {mapping} path="{path}.when" self={col.name} {sources} {detectedFor} placeholder="e.g. [temp] > 30 and [unit] = &quot;C&quot;" />
      {:else}
        <div class="builder">
          <select value={cond.column} onchange={(e) => setPart(i, 'column', e.target.value)}>
            {#if !cond.column}<option value="" disabled>column…</option>{/if}
            {#each readable as c}<option value={c.name}>{c.name}{c.note ? ` (${c.note})` : ''}</option>{/each}
            {#if newSources.length}
              <optgroup label="Source columns (adds a helper)">
                {#each newSources as s}<option value="source:{s.key}">{s.label}</option>{/each}
              </optgroup>
            {/if}
          </select>
          <select value={cond.op} onchange={(e) => setPart(i, 'op', e.target.value)}>
            {#each OPS as [op, label]}<option value={op}>{label}</option>{/each}
          </select>
          {#if cond.op !== 'empty' && cond.op !== 'filled'}
            <input value={cond.value} oninput={(e) => setPart(i, 'value', e.target.value)}
              placeholder={placeholder[M.typeOf(mapping, cond.column)] ?? 'value'} />
          {/if}
        </div>
      {/if}
      <div class="then row">
        <span class="word">then</span>
        {#if k.then_formula !== undefined}
          <div class="grow"><FormulaField bind:value={k.then_formula} {mapping} {path} self={col.name} colType={col.type} {sources} {detectedFor} placeholder="a formula" /></div>
        {:else}
          <input class="grow" bind:value={k.then} placeholder={placeholder[col.type] ?? 'value'} />
        {/if}
        <button class="icon" class:on={k.then_formula !== undefined} title={k.then_formula !== undefined ? 'Use a fixed value' : 'Compute with a formula'}
          onclick={() => thenFormula(k, k.then_formula === undefined)}><Sigma size={13} /></button>
      </div>
      {#each caseIssues(i) as x}<div class="issue {x.severity}">{x.message}</div>{/each}
    </div>
  {/each}
  <button class="add" onclick={add}><Plus size={13} /> Add a case</button>

  <div class="case otherwise">
    <div class="then row">
      <span class="word">Otherwise</span>
      <select value={elseMode_} onchange={(e) => elseMode(e.target.value)}>
        <option value="none">{col.required ? 'error (required)' : 'empty'}</option>
        <option value="value">a value</option>
        <option value="formula">a formula</option>
      </select>
      {#if elseMode_ === 'value'}<input class="grow" bind:value={r.else} placeholder={placeholder[col.type] ?? 'value'} />{/if}
    </div>
    {#if elseMode_ === 'formula'}
      <FormulaField bind:value={r.else_formula} {mapping} {path} self={col.name} colType={col.type} {sources} {detectedFor} placeholder="a formula" />
    {/if}
    {#each issues.filter((x) => x.path.startsWith(`${path}.else`)) as x}<div class="issue {x.severity}">{x.message}</div>{/each}
  </div>
</div>

<style>
  /* Every level may shrink below its content: selects are as wide as
     their longest option otherwise, and the panel would scroll sideways. */
  .cases { display: grid; grid-template-columns: minmax(0, 1fr); gap: 6px; margin-top: 10px; }
  .case { display: grid; grid-template-columns: minmax(0, 1fr); gap: 5px; padding: 8px; border: 1px solid var(--line); border-radius: 8px; background: var(--neutral-soft); min-width: 0; }
  .builder > * { width: 100%; }
  .head { flex-wrap: nowrap; gap: 2px; }
  .head .icon { border: none; background: none; color: var(--muted); padding: 2px 4px; display: inline-flex; }
  .head .icon:hover:not(:disabled) { color: var(--accent); }
  .word { font-size: 11px; font-weight: 700; text-transform: uppercase; letter-spacing: 0.04em; color: var(--muted); }
  .builder { display: grid; grid-template-columns: minmax(0, 1.3fr) auto minmax(0, 1fr); gap: 4px; }
  .builder select, .builder input { min-width: 0; }
  .then { flex-wrap: nowrap; gap: 5px; align-items: flex-start; }
  .then .word { padding-top: 6px; min-width: 32px; }
  .grow { flex: 1; min-width: 0; }
  .then .icon { display: inline-flex; padding: 5px 6px; }
  .then .icon.on { background: var(--soft); color: var(--accent); border-color: var(--accent); }
  .add { justify-self: start; display: inline-flex; gap: 4px; align-items: center; font-size: 12px; }
  .otherwise .word { min-width: auto; }
</style>
