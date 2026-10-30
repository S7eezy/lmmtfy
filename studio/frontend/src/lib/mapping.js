// Helpers editing a mapping (the JSON form of config.Mapping, target
// inline). The editor shows target columns joined with their rules; these
// functions keep both sides consistent.

export const TYPES = ['string', 'integer', 'decimal', 'boolean', 'date', 'datetime', 'time']

// Options that only make sense for some types: cleared when the type changes,
// otherwise validation rejects them.
const TARGET_TYPE_OPTIONS = ['max_length', 'on_overflow', 'case', 'pattern', 'allowed_values', 'min', 'max',
  'scale', 'rounding', 'decimal_sep', 'thousands_sep', 'true_value', 'false_value', 'format', 'timezone']
const RULE_TYPE_OPTIONS = ['formats', 'decimal_sep', 'thousands_sep', 'true_values', 'false_values', 'timezone']

export const columns = (m) => m?.target?.inline?.columns ?? []

export function rule(m, name) {
  m.columns ??= {}
  return m.columns[name]
}

export function uniqueName(m, base) {
  const names = new Set([...columns(m).map((c) => c.name), ...Object.keys(m.helpers ?? {})])
  let name = base
  for (let n = 2; names.has(name); n++) name = `${base}_${n}`
  return name
}

// rename returns false when the name is empty or taken. Formulas follow.
export function rename(m, index, name) {
  const col = columns(m)[index]
  if (col.name === name) return true
  if (!name || columns(m).some((c) => c.name === name) || m.helpers?.[name]) return false
  const r = m.columns?.[col.name]
  if (r) {
    delete m.columns[col.name]
    m.columns[name] = r
  }
  renameRefs(m, col.name, name)
  col.name = name
  return true
}

export function remove(m, index) {
  const [col] = columns(m).splice(index, 1)
  if (m.columns) delete m.columns[col.name]
}

// moveTo moves the column at from so that it ends up at index to.
export function moveTo(m, from, to) {
  const cols = columns(m)
  if (from === to || from < 0 || to < 0 || from >= cols.length || to >= cols.length) return
  const [col] = cols.splice(from, 1)
  cols.splice(to, 0, col)
}

// add appends a column. Without source it needs a default: "" (empty).
export function add(m, source) {
  const base = source ? targetName(source.label) : 'new_column'
  const name = uniqueName(m, base)
  columns(m).push({ name, type: 'string' })
  m.columns ??= {}
  m.columns[name] = source ? sourceRule(source) : { default: '' }
  return columns(m).length - 1
}

function sourceRule(source) {
  return source.index ? { source_index: source.index, if_missing: 'error', if_empty: 'keep', map_unmatched: 'keep' }
    : { source: source.name, if_missing: 'error', if_empty: 'keep', map_unmatched: 'keep' }
}

// setSource points a column at a source column, or at none (added column).
// With what detection found in the new source, the rule reads it right away.
export function setSource(m, name, source, detected = null) {
  const r = (m.columns[name] ??= {})
  const was = sourceKey(r)
  delete r.source
  delete r.source_index
  if (!source) {
    // Without a source, a value comes from a default: the target's, or else
    // an empty one.
    if (columns(m).find((c) => c.name === name)?.default === undefined) r.default ??= ''
    return
  }
  if (source.index) r.source_index = source.index
  else r.source = source.name
  if (r.formula === '') delete r.formula // left by the Formula choice
  const col = columns(m).find((c) => c.name === name) ?? (isPattern(name) ? columns(m).find((c) => matches([c], name).length) : undefined)
  if (col && source.key !== was) readAs(r, col.type, detected)
}

// setType changes a column's type. Options of the old type are cleared;
// when detection read the source as the new type (or almost), its reading
// options (formats, separators, boolean values) are filled in, so the column
// converts right away.
export function setType(m, index, type, detected = null) {
  const col = columns(m)[index]
  if (col.type === type) return
  setColumnType(col, type)
  const r = m.columns?.[col.name]
  if (!r) return
  readAs(r, type, detected)
  if (type === 'decimal' && detected?.type === 'decimal' && detected.scale != null) col.scale = detected.scale
}

// readAs sets how a rule reads its source for a type: the formats,
// separators and boolean values detection found, when it read the source as
// that type (or almost).
function readAs(r, type, detected) {
  for (const k of RULE_TYPE_OPTIONS) delete r[k]
  if (!detected) return
  let read = null
  if (detected.type === type) {
    read = { formats: detected.formats, decimal_sep: detected.decimal_sep, thousands_sep: detected.thousands_sep,
      true_values: detected.true_values, false_values: detected.false_values }
  } else if (detected.near_type === type) {
    read = { formats: detected.near_formats, decimal_sep: detected.near_decimal_sep, thousands_sep: detected.near_thousands_sep }
  } else if (detected.type === 'integer' && (type === 'decimal' || type === 'string')) {
    read = type === 'decimal' ? { thousands_sep: detected.thousands_sep } : null
  }
  for (const [k, v] of Object.entries(read ?? {})) {
    if (v && (!Array.isArray(v) || v.length)) r[k] = v
  }
}

export const clone = (v) => JSON.parse(JSON.stringify(v))

// linkTarget replaces the mapping's target by the content of a target file.
// Rules (from rules, keyed by column name) are kept for target columns of the
// same name; other target columns are mapped to a source column with a
// matching name ("Order date" → order_date). Returns the columns mapped by
// a guess, for the user to check: those whose source is not exactly named
// like the target column. detect(source) returns what detection found in a
// source column, so that rules read it as the target type expects.
export function linkTarget(m, t, rules, sources, detect = () => null) {
  const oldTypes = new Map(columns(m).map((c) => [c.name, c.type]))
  t.output.header ??= true
  m.target.inline = t
  m.columns = {}
  const matched = []
  for (const c of t.columns) {
    const old = rules[c.name]
    if (old) {
      const r = (m.columns[c.name] = clone(old))
      // Read as the target's type, unless it is the type the rule was for.
      if (oldTypes.get(c.name) !== c.type) readAs(r, c.type, detect(sources.find((s) => s.key === sourceKey(r))))
      if (sourceKey(old) !== c.name) matched.push(c.name)
      continue
    }
    const s = sources.find((s) => s.label === c.name) ??
      sources.find((s) => sameName(s.label, c.name))
    if (s) {
      const r = (m.columns[c.name] = sourceRule(s))
      readAs(r, c.type, detect(s))
      if (s.label !== c.name) matched.push(c.name)
    }
  }
  return matched
}

// renameRules follows columns renamed in the target ({ old: new }), in
// rules and formulas.
export function renameRules(m, renames) {
  for (const [from, to] of Object.entries(renames)) {
    if (m.columns?.[from] && !m.columns[to]) {
      m.columns[to] = m.columns[from]
      delete m.columns[from]
    }
    renameRefs(m, from, to)
  }
}

// Every rule and helper, with its path in issues: [path, name, rule].
export function allRules(m) {
  return [...Object.entries(m.columns ?? {}).map(([n, r]) => [`columns.${n}`, n, r]),
    ...Object.entries(m.helpers ?? {}).map(([n, r]) => [`helpers.${n}`, n, r])]
}

// renameRefs rewrites [from] as [to] in every formula. Text in quotes is
// left alone; ] inside a name is written ]].
export function renameRefs(m, from, to) {
  for (const [, , r] of allRules(m)) {
    if (r.formula) r.formula = mapRefs(r.formula, (name) => (name === from ? to : name))
  }
}

export function mapRefs(src, fn) {
  let out = ''
  for (let i = 0; i < src.length;) {
    const c = src[i]
    if (c === '"') {
      let j = i + 1
      while (j < src.length && !(src[j] === '"' && src[j + 1] !== '"')) j += src[j] === '"' ? 2 : 1
      out += src.slice(i, j + 1)
      i = j + 1
    } else if (c === '[') {
      let j = i + 1, name = ''
      while (j < src.length && !(src[j] === ']' && src[j + 1] !== ']')) {
        if (src[j] === ']') j++
        name += src[j++]
      }
      if (j >= src.length) return out + src.slice(i) // not closed: as typed
      const trimmed = name.trim()
      out += `[${fn(trimmed).replaceAll(']', ']]')}]`
      i = j + 1
    } else {
      out += c
      i++
    }
  }
  return out
}

// How a column is filled: from a source column (maybe through a formula),
// a fixed value, a formula, conditions or row numbers.
export function fillMode(r) {
  if (r?.source || r?.source_index) return 'source'
  if (r?.cases) return 'cases'
  if (r?.sequence) return 'sequence'
  if (r?.formula !== undefined) return 'formula'
  return 'fixed'
}

const COMPUTED = ['formula', 'cases', 'else', 'else_formula', 'sequence']

// setFill switches a rule to fixed values, a formula, conditions or row
// numbers; switching to a source is done by choosing one (setSource).
export function setFill(m, name, mode, col) {
  fillRule((m.columns[name] ??= {}), mode, col)
}

// fillRule is setFill on a rule or helper. col is its target column, for
// its default.
export function fillRule(r, mode, col) {
  if (fillMode(r) === mode) return
  delete r.source
  delete r.source_index
  const formula = r.formula
  for (const k of COMPUTED) delete r[k]
  if (r.default === '') delete r.default
  if (mode === 'formula') r.formula = formula ?? ''
  else if (mode === 'cases') r.cases = [{ when: '', then: '' }]
  else if (mode === 'sequence') r.sequence = {}
  else if (col?.default === undefined) r.default ??= ''
}

// Conditions in the shape the builder edits: [column] op value, or an
// empty test. Anything else is edited as formula text.
const OPS = ['=', '<>', '<', '<=', '>', '>=']
export function parseCondition(src) {
  const s = (src ?? '').trim()
  if (!s) return { column: '', op: '=', value: '' }
  let m = s.match(/^(not\s+)?is_empty\(\s*\[((?:[^\]]|\]\])+)\]\s*\)$/i)
  if (m) return { column: m[2].replaceAll(']]', ']').trim(), op: m[1] ? 'filled' : 'empty', value: '' }
  m = s.match(/^\[((?:[^\]]|\]\])+)\]\s*(<>|<=|>=|=|<|>)\s*(.+)$/)
  if (!m) return null
  const raw = m[3].trim()
  let value
  if (/^-?\d+(\.\d+)?$/.test(raw) || /^(true|false)$/i.test(raw)) value = raw.toLowerCase()
  else if (/^"(?:[^"]|"")*"$/.test(raw)) value = raw.slice(1, -1).replaceAll('""', '"')
  else if ((m = raw.match(/^(?:date|datetime)\(\s*"([^"]*)"\s*\)$/i)) && !raw.includes('),')) value = m[1]
  else return null
  const c = s.match(/^\[((?:[^\]]|\]\])+)\]/)[1].replaceAll(']]', ']').trim()
  return { column: c, op: s.match(/\]\s*(<>|<=|>=|=|<|>)/)[1], value }
}

export function buildCondition({ column, op, value }, type) {
  if (!column) return ''
  const ref = `[${column.replaceAll(']', ']]')}]`
  if (op === 'empty') return `is_empty(${ref})`
  if (op === 'filled') return `not is_empty(${ref})`
  let lit = value ?? ''
  if (type === 'integer' || type === 'decimal') lit = lit.trim() || '0'
  else if (type === 'boolean') lit = lit.trim().toLowerCase() === 'false' ? 'false' : 'true'
  else if (type === 'date' || type === 'datetime') lit = `${type}("${lit.trim()}")`
  else lit = `"${lit.replaceAll('"', '""')}"`
  return `${ref} ${OPS.includes(op) ? op : '='} ${lit}`
}

// The type of a column or helper formulas can read, by name.
export function typeOf(m, name) {
  return columns(m).find((c) => c.name === name)?.type ?? m.helpers?.[name]?.type ?? ''
}

// addHelper reads a source column into a new helper, typed as detection
// found it, unless a helper already reads it that way. Returns its name.
export function addHelper(m, source, detected, type) {
  const t = type ?? helperType(detected)
  for (const [n, h] of Object.entries(m.helpers ?? {})) {
    if (sourceKey(h) === source.key && h.type === t && !h.formula) return n
  }
  const name = uniqueName(m, targetName(source.label))
  const h = { type: t, ...sourceRule(source) }
  readAs(h, h.type, detected)
  m.helpers = { ...(m.helpers ?? {}), [name]: h }
  return name
}

// A helper takes the type most values read as: a few bad values are
// errors to see, not a reason to read everything as text.
function helperType(d) {
  if (!d) return 'string'
  if (d.type === 'string') return d.near_type && d.near_confidence >= 0.8 && TYPES.includes(d.near_type) ? d.near_type : 'string'
  return TYPES.includes(d.type) ? d.type : 'string'
}

// setHelperSource points a helper at another source column.
export function setHelperSource(m, name, source, detected) {
  const h = m.helpers[name]
  delete h.source
  delete h.source_index
  if (source.index) h.source_index = source.index
  else h.source = source.name
  readAs(h, h.type, detected)
}

// setHelperType changes a helper's type: reading options follow.
export function setHelperType(m, name, type, detected) {
  const h = m.helpers[name]
  if (h.type === type) return
  h.type = type
  readAs(h, type, detected)
  delete h.default
}

// renameHelper returns false when the name is empty or taken.
export function renameHelper(m, from, to) {
  if (from === to) return true
  if (!to || m.helpers[to] || columns(m).some((c) => c.name === to)) return false
  const helpers = {}
  for (const [n, h] of Object.entries(m.helpers)) helpers[n === from ? to : n] = h
  m.helpers = helpers
  renameRefs(m, from, to)
  return true
}

export function removeHelper(m, name) {
  delete m.helpers[name]
  if (!Object.keys(m.helpers).length) delete m.helpers
}

// Rules whose column the target does not have (renamed or removed there).
export const orphans = (m) => Object.keys(m?.columns ?? {}).filter((n) => !isPattern(n) && !columns(m).some((c) => c.name === n))

// outputSummary describes a target's file format in a few words.
export function outputSummary(o) {
  if (!o) return ''
  const d = { ',': 'comma', ';': 'semicolon', '\t': 'tab', tab: 'tab', '|': 'pipe' }[o.delimiter] ?? `“${o.delimiter}”`
  return [o.encoding ?? 'utf-8', d, (o.line_ending ?? 'crlf').toUpperCase(), o.header === false ? 'no header' : 'header'].join(' · ')
}

export const blankTarget = () => ({
  version: 1, name: '',
  output: { encoding: 'utf-8', delimiter: ',', quote: '"', quote_style: 'minimal', escape: 'double', line_ending: 'crlf', header: true },
  columns: [{ name: 'column_1', type: 'string' }],
})

// setColumnType changes a target column's type, clearing the options (and
// default) of the old type.
export function setColumnType(col, type) {
  if (col.type === type) return
  col.type = type
  for (const k of TARGET_TYPE_OPTIONS) delete col[k]
  delete col.default
}

// sources lists the source columns of the input: header names (trimmed,
// duplicates numbered like the engine does) or positions without header.
export function sources(raw, header) {
  if (!raw?.rows) return []
  if (!header) {
    return Array.from({ length: raw.width }, (_, i) => ({ index: i + 1, label: `#${i + 1}`, key: `#${i + 1}` }))
  }
  const row = raw.rows.find((r) => r.kind === 'header')
  if (!row) return []
  const trimmed = row.fields.map((f) => f.trim())
  const seen = new Map()
  return trimmed.map((h) => {
    let name = h
    const n = (seen.get(h) ?? 0) + 1
    seen.set(h, n)
    if (n > 1) {
      for (let k = n; ; k++) {
        const c = `${h}_${k}`
        if (!trimmed.includes(c) && !seen.has(c)) {
          name = c
          seen.set(c, 1)
          break
        }
      }
    }
    return { name, label: name, key: name }
  })
}

export function sourceKey(r) {
  if (!r) return ''
  if (r.source_index) return `#${r.source_index}`
  return r.source ?? ''
}

// targetName mirrors detect.TargetName: "N° Commande" → "N_Commande".
export function targetName(header) {
  const plain = header.normalize('NFD').replace(/\p{Mn}/gu, '')
  let name = plain.replace(/[^A-Za-z0-9]+/g, '_').replace(/^_+|_+$/g, '')
  if (!name) name = 'column'
  if (/^[0-9]/.test(name)) name = 'c_' + name
  return name
}

// sameName reports whether a source header and a target column name are
// the same name, ignoring case and punctuation.
function sameName(header, name) {
  return targetName(header).toLowerCase() === targetName(name).toLowerCase()
}

export function issuesFor(issues, name, helper = false) {
  if (helper) return (issues ?? []).filter((i) => i.path === `helpers.${name}` || i.path.startsWith(`helpers.${name}.`))
  return (issues ?? []).filter((i) => i.path === `columns.${name}` || i.path.startsWith(`columns.${name}.`) ||
    i.path === `target.columns.${name}` || i.path.startsWith(`target.columns.${name}.`))
}

export const list = (s) => (s ?? '').split(',').map((v) => v.trim()).filter((v) => v !== '')
export const joinList = (a) => (a ?? []).join(', ')

// usesAggregates reports whether a formula of the mapping aggregates
// values over the file (lowest, total, ...).
export function usesAggregates(m) {
  const agg = /\b(first|last|lowest|highest|total|mean|count)\s*\(/i
  return allRules(m).some(([, , r]) => [r.formula, r.default_formula, r.else_formula, ...(r.cases ?? []).flatMap((k) => [k.when, k.then_formula])]
    .some((f) => f && agg.test(M_stripText(f))))
}

// Text in quotes is not code.
const M_stripText = (f) => f.replace(/"(?:[^"]|"")*"/g, '""')

// Combining rows. A pattern (price_{n}) fills the numbered target columns
// price_0, price_1, ... with the 1st, 2nd, ... row of a batch.
export const SLOT = '{n}'
export const isPattern = (key) => key.includes(SLOT)

const escapeRe = (s) => s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')

// The columns a pattern matches, with their numbers.
export function matches(cols, pattern) {
  const [before, after] = pattern.split(SLOT)
  const re = new RegExp(`^${escapeRe(before)}(\\d+)${escapeRe(after ?? '')}$`)
  return cols.flatMap((c) => {
    const m = c.name.match(re)
    return m ? [{ name: c.name, n: Number(m[1]) }] : []
  })
}

// slotsOf maps each target column a pattern fills to that pattern.
export function slotsOf(m) {
  const out = new Map()
  for (const key of Object.keys(m.columns ?? {})) {
    if (!isPattern(key)) continue
    for (const { name } of matches(columns(m), key)) if (!m.columns[name]) out.set(name, key)
  }
  return out
}

// The rule filling a target column: its own or its pattern's.
export function ruleOf(m, name, slots = slotsOf(m)) {
  return m.columns?.[name] ?? (slots.has(name) ? m.columns[slots.get(name)] : undefined)
}

// families finds numbered columns repeated together in a target
// (object_0, price_0, ..., object_16, price_16): the patterns a combined
// mapping would use, sharing the same numbers.
export function families(cols) {
  const byPattern = new Map()
  for (const c of cols) {
    const m = c.name.match(/^(.*?)(\d+)(\D*)$/)
    if (!m) continue
    const pattern = m[1] + SLOT + m[3]
    if (!byPattern.has(pattern)) byPattern.set(pattern, [])
    byPattern.get(pattern).push(Number(m[2]))
  }
  const sets = new Map()
  for (const [pattern, numbers] of byPattern) {
    if (numbers.length < 2) continue
    const key = [...numbers].sort((a, b) => a - b).join(',')
    if (!sets.has(key)) sets.set(key, [])
    sets.get(key).push(pattern)
  }
  // The largest set of patterns sharing their numbers.
  let best = null
  for (const [key, patterns] of sets) {
    const numbers = key.split(',').map(Number)
    if (!best || patterns.length * numbers.length > best.patterns.length * best.numbers.length) best = { patterns, numbers }
  }
  return best ?? { patterns: [], numbers: [] }
}

// combine turns a mapping into a combined one: group settings, and one
// pattern rule per family, from the rule of its first column or a source
// named like the pattern (price_{n} ← price).
export function combine(m, group, patterns, sources = [], detect = () => null) {
  m.group = group
  for (const pattern of patterns) {
    const cols = matches(columns(m), pattern).sort((a, b) => a.n - b.n)
    if (!m.columns[pattern]) {
      const first = cols.map((c) => m.columns[c.name]).find(Boolean)
      const s = sources.find((x) => sameName(x.label, pattern.replace(SLOT, '')))
      const col = columns(m).find((c) => c.name === cols[0]?.name)
      m.columns[pattern] = first ? clone(first) : s ? sourceRule(s) : {}
      if (!first && s && col) readAs(m.columns[pattern], col.type, detect(s))
    }
    for (const c of cols) delete m.columns[c.name]
  }
}

// uncombine gives every column of a pattern its own copy of the rule.
export function uncombine(m) {
  delete m.group
  for (const key of Object.keys(m.columns ?? {})) {
    if (!isPattern(key)) continue
    for (const { name } of matches(columns(m), key)) m.columns[name] ??= clone(m.columns[key])
    delete m.columns[key]
  }
}
