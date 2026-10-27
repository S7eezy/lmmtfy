// How sure detection is about a source column, as a level for colours:
//   high  every sampled value fits its type
//   good  ≥ 98% fit (a few odd values)
//   fair  ≥ 95% fit (the detection threshold)
//   low   text that is almost typed (e.g. 80% dates): worth a look
//   text  plain text, nothing to check
export function confidence(d) {
  if (!d) return null
  if (d.type === 'string') {
    if (d.near_type) return { level: 'low', type: d.near_type, share: d.near_confidence, label: `text · ${pct(d.near_confidence)} ${d.near_type}` }
    return { level: 'text', type: 'string', share: 1, label: 'text' }
  }
  const level = d.confidence >= 0.9995 ? 'high' : d.confidence >= 0.98 ? 'good' : 'fair'
  return { level, type: d.type, share: d.confidence, label: `${d.type} · ${pct(d.confidence)}` }
}

export const pct = (x) => {
  const p = x * 100
  return (p >= 99.95 || p < 10 ? Math.round(p) : Math.floor(p * 10) / 10) + '%'
}
