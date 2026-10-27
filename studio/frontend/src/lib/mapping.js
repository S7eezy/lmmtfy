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
  const names = new Set(columns(m).map((c) => c.name))
  let name = base
  for (let n = 2; names.has(name); n++) name = `${base}_${n}`
  return name
}

// rename returns false when the name is empty or taken.
export function rename(m, index, name) {
  const col = columns(m)[index]
  if (col.name === name) return true
  if (!name || columns(m).some((c) => c.name === name)) return false
  const r = m.columns?.[col.name]
  if (r) {
    delete m.columns[col.name]
    m.columns[name] = r
  }
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
    r.default ??= ''
    return
  }
  if (source.index) r.source_index = source.index
  else r.source = source.name
  const col = columns(m).find((c) => c.name === name)
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
      sources.find((s) => targetName(s.label) === targetName(c.name) || s.label.toLowerCase() === c.name.toLowerCase())
    if (s) {
      const r = (m.columns[c.name] = sourceRule(s))
      readAs(r, c.type, detect(s))
      if (s.label !== c.name) matched.push(c.name)
    }
  }
  return matched
}

// renameRules follows columns renamed in the target ({ old: new }).
export function renameRules(m, renames) {
  for (const [from, to] of Object.entries(renames)) {
    if (m.columns?.[from] && !m.columns[to]) {
      m.columns[to] = m.columns[from]
      delete m.columns[from]
    }
  }
}

// Rules whose column the target does not have (renamed or removed there).
export const orphans = (m) => Object.keys(m?.columns ?? {}).filter((n) => !columns(m).some((c) => c.name === n))

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

// targetName mirrors detect.TargetName: "N° Commande" → "n_commande".
export function targetName(header) {
  const plain = header.normalize('NFD').replace(/\p{Mn}/gu, '').toLowerCase()
  let name = plain.replace(/[^a-z0-9]+/g, '_').replace(/^_+|_+$/g, '')
  if (!name) name = 'column'
  if (/^[0-9]/.test(name)) name = 'c_' + name
  return name
}

export function issuesFor(issues, name) {
  return (issues ?? []).filter((i) => i.path === `columns.${name}` || i.path.startsWith(`columns.${name}.`) ||
    i.path === `target.columns.${name}` || i.path.startsWith(`target.columns.${name}.`))
}

export const list = (s) => (s ?? '').split(',').map((v) => v.trim()).filter((v) => v !== '')
export const joinList = (a) => (a ?? []).join(', ')
