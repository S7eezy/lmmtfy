<script>
  // Saved tests: rows of a sample with the output they must give. They are
  // checked after every change; lmmtfy test replays them.
  import { CircleCheck, CircleX, LoaderCircle, Trash2, ChevronRight, Pin, RefreshCcw } from '@lucide/svelte'

  let { mapping, results = null, hasErrors = false } = $props()

  const tests = $derived(mapping.tests ?? [])
  const resultOf = (t) => (results ?? []).find((r) => r.name === t.name)
  let open = $state(new Set())
  function toggle(i) {
    const next = new Set(open)
    if (next.has(i)) next.delete(i)
    else next.add(i)
    open = next
  }
  function remove(i) {
    mapping.tests.splice(i, 1)
    if (!mapping.tests.length) delete mapping.tests
    open = new Set()
  }
  function rename(t, name) {
    name = name.trim()
    if (!name || tests.some((x) => x !== t && x.name === name)) return false
    t.name = name
    return true
  }
  // The mapping changed on purpose: what it gives now is what is expected.
  function accept(t, r) {
    t.output = r.output ?? []
    if (r.errors?.length) t.errors = [...r.errors]
    else delete t.errors
  }
  // Columns of a list of rows, in order of appearance.
  const columnsOf = (rows) => [...new Set((rows ?? []).flatMap((row) => row.map((c) => c.column)))]
  const cell = (row, col) => row.find((c) => c.column === col)?.value
</script>

<div class="tests">
  {#if !tests.length}
    <p class="empty muted"><Pin size={14} /> Pin a row of the Output or Rejected tab to save it as a test: Studio checks it after every change, so a change that alters its result shows here. <span class="mono">lmmtfy test</span> replays tests too.</p>
  {:else if hasErrors}
    <p class="empty muted">Tests run once the mapping has no errors.</p>
  {/if}
  {#each tests as t, i (t)}
    {@const r = resultOf(t)}
    <div class="test" class:failed={r && !r.passed}>
      <div class="head row">
        <button class="icon caret" class:open={open.has(i)} onclick={() => toggle(i)} aria-label="Details"><ChevronRight size={14} /></button>
        <span class="state">
          {#if hasErrors || !results}<LoaderCircle size={15} />
          {:else if r?.passed}<span class="ok"><CircleCheck size={15} /></span>
          {:else}<span class="ko"><CircleX size={15} /></span>{/if}
        </span>
        <input class="name" value={t.name} onchange={(e) => { if (!rename(t, e.target.value)) e.target.value = t.name }} />
        <span class="muted small">{t.input.length} input {t.input.length === 1 ? 'row' : 'rows'} → {(t.output ?? []).length} output {(t.output ?? []).length === 1 ? 'row' : 'rows'}{t.errors?.length ? `, ${t.errors.length} ${t.errors.length === 1 ? 'error' : 'errors'}` : ''}</span>
        <span class="spacer"></span>
        {#if r && !r.passed}<button class="small-btn" onclick={() => accept(t, r)} title="Expect what the mapping gives now"><RefreshCcw size={12} /> Expect the new result</button>{/if}
        <button class="icon delete" onclick={() => remove(i)} aria-label="Delete test" title="Delete the test"><Trash2 size={14} /></button>
      </div>
      {#if r && !r.passed}
        <ul class="problems">{#each r.problems ?? [] as p}<li>{p}</li>{/each}</ul>
      {/if}
      {#if open.has(i)}
        <div class="detail">
          {#each [['Input', t.input], ['Expected output', t.output ?? []], ...(r && !r.passed ? [['Output now', r.output ?? []]] : [])] as [label, rows]}
            {@const columns = columnsOf(rows)}
            <div class="block">
              <span class="label muted">{label}</span>
              {#if rows.length}
                <table>
                  <thead><tr>{#each columns as c}<th>{c}</th>{/each}</tr></thead>
                  <tbody>{#each rows as row}<tr>{#each columns as c}{@const v = cell(row, c)}<td class:unset={v === undefined}>{v ?? '·'}</td>{/each}</tr>{/each}</tbody>
                </table>
              {:else}<span class="muted small">no row</span>{/if}
            </div>
          {/each}
          {#if t.errors?.length}<div class="block"><span class="label muted">Expected errors</span>{#each t.errors as e}<div class="mono small">{e}</div>{/each}</div>{/if}
        </div>
      {/if}
    </div>
  {/each}
</div>

<style>
  .tests { padding: 8px 12px; display: grid; gap: 5px; }
  .empty { display: flex; gap: 7px; align-items: center; margin: 4px 0; }
  .test { border-radius: 7px; background: var(--neutral-soft); padding: 4px 8px; }
  .test.failed { background: var(--error-soft); }
  .head { gap: 7px; align-items: center; }
  .caret { border: none; background: none; padding: 2px; color: var(--muted); transition: transform 0.12s; }
  .caret.open { transform: rotate(90deg); }
  .state { display: inline-flex; color: var(--muted); }
  .ok { color: var(--high); display: inline-flex; }
  .ko { color: var(--error); display: inline-flex; }
  .name { font-weight: 600; border-color: transparent; background: transparent; padding: 2px 6px; min-width: 120px; width: 260px; }
  .name:hover, .name:focus { border-color: var(--line); background: var(--panel); }
  .small { font-size: 12px; }
  .small-btn { display: inline-flex; gap: 4px; align-items: center; font-size: 12px; padding: 2px 9px; white-space: nowrap; }
  .delete { color: var(--muted); }
  .delete:hover { color: var(--error); }
  .problems { margin: 2px 0 4px 52px; padding: 0; list-style: none; font-size: 12px; color: var(--error); }
  .detail { display: grid; gap: 8px; margin: 4px 0 6px 52px; }
  .block { display: grid; gap: 3px; }
  .label { font-size: 11px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.03em; }
  table { border-collapse: collapse; font-family: var(--mono); font-size: 12px; justify-self: start; }
  th, td { border: 1px solid var(--line); padding: 2px 8px; text-align: left; white-space: pre; }
  th { background: var(--panel); font-weight: 600; }
  td.unset { color: var(--muted); }
</style>
