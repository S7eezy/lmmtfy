<script>
  // A formula, as typed: autocomplete for columns and functions, the error
  // where it is, what the formula gives, and how it computes the first rows
  // of the sample.
  //
  // Typing [ lists the columns a formula can read: output columns, helpers,
  // and source columns, which become helpers when picked (a formula reads
  // typed values, and a helper is how a source column gets a type).
  import { tick } from 'svelte'
  import { Lightbulb, CircleAlert, Equal } from '@lucide/svelte'
  import TypeIcon from './TypeIcon.svelte'
  import * as M from './mapping.js'
  import * as api from './api.js'

  let { value = $bindable(''), mapping, path, self = '', hasValue = false, colType = '', sources = [], detectedFor = () => null,
    trace = null, examples = [], problems = [], placeholder = '' } = $props()

  let ta = $state(null)
  let menu = $state(null) // { items, index, from, to }
  let result = $state(null) // { path, ...check }
  // A result for another column (the field is reused when the selection
  // changes) is never shown, even for the time the new check takes.
  const check = $derived(result?.path === path ? result : null)
  let docs = $state([])
  let showExamples = $state(false)
  api.functions().then((d) => (docs = d ?? []))

  // Check as it is typed. The mapping is read whole, so a change elsewhere
  // (a renamed column, a new helper) re-checks too.
  $effect(() => {
    const src = value ?? ''
    const snapshot = JSON.parse(JSON.stringify(mapping))
    const p = path
    if (!src.trim()) { result = null; return }
    const t = setTimeout(() => api.checkFormula(snapshot, p, src).then((c) => (result = { ...c, path: p })).catch(() => {}), 150)
    return () => clearTimeout(t)
  })

  const TYPE_WORDS = { string: 'text', integer: 'a whole number', decimal: 'a decimal number', boolean: 'true or false', date: 'a date', datetime: 'a datetime', time: 'a time', empty: 'nothing (always empty)' }

  // What the cursor is in: a column name after [, or a function name.
  function context() {
    const pos = ta?.selectionStart ?? 0
    const before = (value ?? '').slice(0, pos)
    if ((before.match(/"/g) ?? []).length % 2) return null // inside text
    const open = before.lastIndexOf('['), close = before.lastIndexOf(']')
    if (open > close) return { kind: 'column', query: before.slice(open + 1).trim().toLowerCase(), from: open, to: pos }
    const word = before.match(/[A-Za-z_][A-Za-z0-9_]*$/)
    if (word) return { kind: 'name', query: word[0].toLowerCase(), from: pos - word[0].length, to: pos }
    return null
  }

  function columnItems(q) {
    const outputs = [], helpers = [], fresh = []
    // Combined rows: numbered columns stand for their pattern, and helpers
    // (one value per row of a batch) are what aggregates and patterns read.
    const slots = mapping.group ? M.slotsOf(mapping) : new Map()
    for (const c of M.columns(mapping)) {
      if (c.name !== self && !slots.has(c.name)) outputs.push({ label: c.name, type: c.type, note: 'output column', insert: `[${c.name}]` })
    }
    for (const [n, h] of Object.entries(mapping.helpers ?? {})) {
      if (n !== self) helpers.push({ label: n, type: h.type, note: mapping.group ? 'helper · one value per row of a batch' : 'helper', insert: `[${n}]` })
    }
    const helperSources = new Set(Object.values(mapping.helpers ?? {}).map((h) => M.sourceKey(h)))
    for (const s of sources) {
      if (helperSources.has(s.key)) continue
      const d = detectedFor(s)
      fresh.push({ label: s.label, type: d?.type ?? 'string', note: 'source column · adds a helper', source: s })
    }
    const items = mapping.group ? [...helpers, ...fresh, ...outputs] : [...outputs, ...helpers, ...fresh]
    return items.filter((i) => i.label.toLowerCase().includes(q)).slice(0, 14)
  }

  function nameItems(q) {
    const items = []
    if (hasValue && 'value'.startsWith(q)) items.push({ label: 'value', note: 'the source value of this column', insert: 'value' })
    for (const d of docs) {
      if (d.name.startsWith(q)) items.push({ label: d.name, args: d.args, note: d.summary, insert: d.args === '()' ? `${d.name}()` : `${d.name}(`, fn: true })
    }
    return items.filter((i) => i.label !== q || i.fn).slice(0, 12)
  }

  function refresh() {
    const ctx = context()
    if (!ctx || (ctx.kind === 'name' && !ctx.query)) { menu = null; return }
    const items = ctx.kind === 'column' ? columnItems(ctx.query) : nameItems(ctx.query)
    menu = items.length ? { ...ctx, items, index: Math.min(menu?.index ?? 0, items.length - 1) } : null
  }

  async function accept(item) {
    let text = item.insert
    if (item.source) {
      const name = M.addHelper(mapping, item.source, detectedFor(item.source))
      text = `[${name}]`
    }
    const { from, to } = menu
    // A column name typed after [ may already be closed: replace up to the ].
    let end = to
    if (text.startsWith('[') && value.slice(to).match(/^[^\[\]"]*\]/)) end = to + value.slice(to).indexOf(']') + 1
    value = value.slice(0, from) + text + value.slice(end)
    menu = null
    const caret = from + text.length
    await tick()
    ta?.focus()
    ta?.setSelectionRange(caret, caret)
  }

  function onkeydown(e) {
    if (!menu) return
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault()
      const n = menu.items.length
      menu.index = (menu.index + (e.key === 'ArrowDown' ? 1 : n - 1)) % n
    } else if (e.key === 'Enter' || e.key === 'Tab') {
      e.preventDefault()
      accept(menu.items[menu.index])
    } else if (e.key === 'Escape') {
      e.stopPropagation()
      menu = null
    }
  }

  // The function the cursor is in, for its signature.
  let caretPos = $state(0)
  const inside = $derived.by(() => {
    const before = (value ?? '').slice(0, caretPos)
    let depth = 0
    for (let i = before.length - 1; i >= 0; i--) {
      const c = before[i]
      if (c === ')') depth++
      else if (c === '(') {
        if (depth === 0) {
          const name = before.slice(0, i).match(/([A-Za-z_][A-Za-z0-9_]*)\s*$/)?.[1]?.toLowerCase()
          return docs.find((d) => d.name === name) ?? null
        }
        depth--
      }
    }
    return null
  })

  async function insert(text) {
    const s = ta?.selectionStart ?? (value ?? '').length, e = ta?.selectionEnd ?? s
    value = (value ?? '').slice(0, s) + text + (value ?? '').slice(e)
    showExamples = false
    await tick()
    ta?.focus()
    ta?.setSelectionRange(s + text.length, s + text.length)
  }

  // Mapping issues on this formula that the field's own check cannot see:
  // the others it shows already, and sooner.
  const wholeMapping = (p) => p.severity === 'warning' || p.message.startsWith('circular reference')

  // The error's position, as before / at / after for highlighting.
  const where = $derived.by(() => {
    if (!check?.pos || !value) return null
    const chars = [...value]
    const i = check.pos - 1
    return { before: chars.slice(0, i).join(''), at: chars[i] ?? ' ', after: chars.slice(i + 1).join('') }
  })
  const rows = $derived(trace ?? [])
  const inputs = $derived(rows.find((r) => r.inputs?.length)?.inputs?.map((i) => i.column) ?? [])
  const lines = $derived(Math.min(6, Math.max(2, (value ?? '').split('\n').length)))
</script>

<div class="formula">
  <div class="field">
    <textarea bind:this={ta} class="mono" rows={lines} spellcheck="false" {placeholder}
      bind:value oninput={refresh} onkeydown={onkeydown} onclick={refresh}
      onkeyup={(e) => { caretPos = ta.selectionStart; if (e.key === 'ArrowLeft' || e.key === 'ArrowRight') refresh() }}
      onselect={() => (caretPos = ta.selectionStart)} onblur={() => setTimeout(() => (menu = null), 150)}></textarea>
    {#if menu}
      <ul class="menu" role="listbox">
        {#each menu.items as item, i}
          <li role="option" aria-selected={i === menu.index}>
            <button class:on={i === menu.index} onmousedown={(e) => { e.preventDefault(); accept(item) }}>
              {#if item.type}<TypeIcon type={item.type} />{/if}
              <span class="mono label">{item.fn ? item.label : item.insert ?? item.label}{#if item.args}<span class="args">{item.args}</span>{/if}</span>
              <span class="note">{item.note}</span>
            </button>
          </li>
        {/each}
      </ul>
    {/if}
  </div>

  <div class="status row">
    {#if check?.error}
      <span class="err"><CircleAlert size={13} /> {check.error}</span>
    {:else if check?.type}
      <span class="gives"><Equal size={13} /> gives {TYPE_WORDS[check.type] ?? check.type}{#if colType && check.type !== colType && check.type !== 'empty'}, written as {colType}{/if}</span>
    {:else if !inside}
      <span class="muted">Type <kbd>[</kbd> for columns, letters for functions.</span>
    {/if}
    <span class="spacer"></span>
    {#if examples.length}
      <button class="link" onclick={() => (showExamples = !showExamples)}><Lightbulb size={13} /> Examples</button>
    {/if}
  </div>
  {#if !check?.error}{#each problems.filter(wholeMapping) as p}<div class="issue {p.severity}">{p.message}</div>{/each}{/if}
  {#if where}<pre class="context mono">{where.before}<mark>{where.at}</mark>{where.after}</pre>{/if}
  {#if inside}<p class="signature"><span class="mono">{inside.name}{inside.args}</span> — {inside.summary}</p>{/if}

  {#if showExamples}
    <div class="examples">
      {#each examples as ex}
        <button onclick={() => (ex.apply ? (ex.apply(), (showExamples = false)) : insert(ex.formula))}>
          <span class="title">{ex.label}</span>
          {#if ex.formula}<span class="mono">{ex.formula}</span>{/if}
          {#if ex.note}<span class="muted">{ex.note}</span>{/if}
        </button>
      {/each}
    </div>
  {/if}

  {#if rows.length}
    <div class="trace">
      <table>
        <thead>
          <tr>
            <th>line</th>
            {#if hasValue}<th>value</th>{/if}
            {#each inputs as n}<th class="mono">[{n}]</th>{/each}
            <th class="result">result</th>
          </tr>
        </thead>
        <tbody>
          {#each rows as t}
            <tr class:failed={t.error && !t.result}>
              <td class="muted">{t.line}</td>
              {#if hasValue}<td class:empty={!t.value}>{t.value || 'empty'}</td>{/if}
              {#each inputs as n, k}
                {@const input = t.inputs?.[k]}
                <td class:empty={!input || input.empty}>{input && !input.empty ? input.value : 'empty'}</td>
              {/each}
              <td class="result" class:empty={!t.result && !t.error} title={t.error ?? ''}>{t.error && !t.result ? t.error : t.result || 'empty'}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</div>

<style>
  .formula { display: grid; gap: 4px; }
  .field { position: relative; }
  textarea { width: 100%; resize: vertical; line-height: 1.5; padding: 6px 8px; }
  .menu { position: absolute; z-index: 30; left: 0; right: 0; top: calc(100% + 2px); margin: 0; padding: 4px; list-style: none;
    background: var(--panel); border: 1px solid var(--line); border-radius: 8px; box-shadow: 0 6px 20px rgb(0 0 0 / 0.14); max-height: 260px; overflow: auto; }
  .menu button { display: flex; gap: 6px; align-items: baseline; width: 100%; border: none; background: none; padding: 4px 6px; text-align: left; border-radius: 5px; }
  .menu button.on { background: var(--soft); }
  .menu .label { white-space: nowrap; font-weight: 600; }
  .menu .args { font-weight: 400; color: var(--muted); }
  .menu .note { color: var(--muted); font-size: 11px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; min-width: 0; }
  .status { font-size: 12px; min-height: 20px; flex-wrap: nowrap; }
  .status button { display: inline-flex; gap: 4px; align-items: center; font-size: 12px; white-space: nowrap; }
  .err, .gives { display: inline-flex; gap: 4px; align-items: center; }
  .err { color: var(--error); }
  .gives { color: var(--ok); }
  .context { margin: 0; padding: 4px 8px; background: var(--error-soft); border-radius: 5px; white-space: pre-wrap; word-break: break-all; }
  .context mark { background: var(--error); color: #fff; border-radius: 2px; }
  .signature { margin: 0; font-size: 12px; color: var(--muted); }
  .signature .mono { color: var(--text); }
  .examples { display: grid; gap: 4px; padding: 6px; background: var(--neutral-soft); border-radius: 7px; }
  .examples button { display: grid; gap: 1px; text-align: left; padding: 5px 8px; background: var(--panel); }
  .examples .title { font-weight: 600; font-size: 12px; }
  .examples .muted { font-size: 11px; }
  .trace { margin-top: 6px; overflow-x: auto; border: 1px solid var(--line); border-radius: 7px; }
  .trace table { border-collapse: collapse; width: 100%; font-size: 12px; }
  .trace th { text-align: left; color: var(--muted); font-weight: 600; font-size: 11px; padding: 4px 6px; background: var(--neutral-soft); white-space: nowrap; }
  .trace td { padding: 3px 6px; border-top: 1px solid var(--line); font-family: var(--mono); white-space: nowrap; max-width: 140px; overflow: hidden; text-overflow: ellipsis; }
  .trace .result { font-weight: 600; }
  .trace td.empty { color: var(--muted); font-style: italic; font-family: inherit; font-weight: 400; }
  .trace tr.failed td.result { color: var(--error); font-family: inherit; font-weight: 500; }
</style>
