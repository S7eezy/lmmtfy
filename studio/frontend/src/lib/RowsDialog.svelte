<script>
  // Which rows are written: a condition keeping rows (keep_rows_when), and
  // duplicate rows dropped by key (duplicates). Changes apply as they are
  // made, like the column panel, so the preview follows.
  import { untrack } from 'svelte'
  import { ListFilter, CopyX } from '@lucide/svelte'
  import Modal from './Modal.svelte'
  import Section from './Section.svelte'
  import FormulaField from './FormulaField.svelte'
  import * as M from './mapping.js'

  let { mapping = $bindable(), issues = [], sources = [], detectedFor = () => null, preview = null, onclose } = $props()

  const cols = $derived(M.columns(mapping).filter((c) => !mapping.columns?.[c.name]?.sequence))
  const keepProblems = $derived(issues.filter((i) => i.path === 'keep_rows_when'))
  const dupProblems = $derived(issues.filter((i) => i.path.startsWith('duplicates')))
  const sum = $derived(preview?.summary)

  // The condition field starts from the mapping as it is when opened.
  let filtering = $state(untrack(() => mapping.keep_rows_when !== undefined))
  let src = $state(untrack(() => mapping.keep_rows_when ?? ''))
  $effect(() => {
    if (filtering) mapping.keep_rows_when = src
    else delete mapping.keep_rows_when
  })

  const examples = [
    { label: 'Drop test rows', formula: '[status] <> "TEST"' },
    { label: 'Keep rows with a value', formula: 'not is_empty([column])' },
    { label: 'Keep a date range', formula: '[date] >= date("2024-01-01")' },
    { label: 'Keep rows without status too', formula: 'is_empty([status]) or [status] <> "CANCELLED"' },
  ]

  function toggleDuplicates(on) {
    if (on) mapping.duplicates = { key: [], keep: 'first' }
    else delete mapping.duplicates
  }
  function toggleKey(name, on) {
    const key = mapping.duplicates.key.filter((k) => k !== name)
    if (on) key.push(name)
    // Key columns in output order.
    mapping.duplicates.key = cols.map((c) => c.name).filter((n) => key.includes(n))
  }
</script>

<Modal title="Rows" icon={ListFilter} {onclose} wide>
  <Section icon={ListFilter} title="Keep rows" hint="Rows where the condition is false are dropped before anything else is computed: no errors, not in the reject file.">
    <label class="inline"><input type="checkbox" bind:checked={filtering} /> Keep only the rows where a condition is true</label>
    {#if filtering}
      <FormulaField bind:value={src} {mapping} path="keep_rows_when" colType="boolean" {sources} {detectedFor} {examples} problems={keepProblems}
        placeholder={'e.g. [status] <> "TEST"'} />
      <p class="hint">A comparison with an empty value is false, so an empty status drops the row in <span class="mono">[status] &lt;&gt; "TEST"</span>; write <span class="mono">is_empty([status]) or …</span> to keep it.
        {#if mapping.group}When rows are combined, the condition reads helpers, for each input row: dropped rows take no slot and no part in totals.{/if}</p>
      {#if sum}<p class="hint">In the preview: {sum.rows_filtered ?? 0} of {sum.rows_read} rows dropped.</p>{/if}
    {/if}
  </Section>

  <Section icon={CopyX} title="Duplicate rows" hint="Output rows with the same values in the key columns, as written. Rejected rows do not count.">
    <label class="inline"><input type="checkbox" checked={!!mapping.duplicates} onchange={(e) => toggleDuplicates(e.target.checked)} /> Drop duplicate rows</label>
    {#if mapping.duplicates}
      <p class="muted small">Rows are duplicates when they have the same</p>
      <div class="keys">
        {#each cols as c (c.name)}
          <label class="key" class:on={mapping.duplicates.key.includes(c.name)}>
            <input type="checkbox" checked={mapping.duplicates.key.includes(c.name)} onchange={(e) => toggleKey(c.name, e.target.checked)} /> {c.name}
          </label>
        {/each}
      </div>
      <div class="choices">
        {#each [['first', 'Keep the first', 'The first row of each key is written; later ones are dropped.'],
                ['last', 'Keep the last', 'For exports that append updates: the latest row wins. The file is read twice.']] as [v, t, d]}
          <button class="choice" class:on={mapping.duplicates.keep === v} onclick={() => (mapping.duplicates.keep = v)}>
            <span><span class="title">{t}</span><span class="desc">{d}</span></span>
          </button>
        {/each}
      </div>
      {#each dupProblems as i}<div class="issue {i.severity}">{i.message}</div>{/each}
      {#if sum?.rows_duplicate}<p class="hint">In the preview: {sum.rows_duplicate} duplicate {sum.rows_duplicate === 1 ? 'row' : 'rows'} dropped.</p>{/if}
    {/if}
  </Section>
  {#snippet footer()}<button class="primary" onclick={onclose}>Done</button>{/snippet}
</Modal>

<style>
  .keys { display: flex; flex-wrap: wrap; gap: 5px; margin: 4px 0 10px; max-height: 160px; overflow: auto; }
  .key { flex-direction: row; display: inline-flex; gap: 4px; align-items: center; border: 1px solid var(--line); border-radius: 12px; padding: 2px 10px 2px 6px; font-size: 12px; font-family: var(--mono); cursor: pointer; }
  .key.on { border-color: var(--accent); background: var(--soft); }
  .choices { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; }
  .small { font-size: 12px; margin: 8px 0 0; }
</style>
