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

export function move(m, index, delta) {
  const cols = columns(m)
  const to = index + delta
  if (to < 0 || to >= cols.length) return
  ;[cols[index], cols[to]] = [cols[to], cols[index]]
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
export function setSource(m, name, source) {
  const r = (m.columns[name] ??= {})
  delete r.source
  delete r.source_index
  if (!source) {
    r.default ??= ''
    return
  }
  if (source.index) r.source_index = source.index
  else r.source = source.name
}

export function setType(m, index, type) {
  const col = columns(m)[index]
  if (col.type === type) return
  col.type = type
  for (const k of TARGET_TYPE_OPTIONS) delete col[k]
  const r = m.columns?.[col.name]
  if (r) for (const k of RULE_TYPE_OPTIONS) delete r[k]
  if (col.default !== undefined) delete col.default
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
