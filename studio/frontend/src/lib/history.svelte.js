// Undo and redo for the document being edited: a mapping, or a target.
//
// The history keeps snapshots (JSON text) of the document. A change is
// recorded once editing pauses, so typing a word is one step. Undoing
// patches the document in place instead of replacing it: components keep
// the objects they hold (a column's rule, a target column tracked by
// identity), and list items with a name (target columns) are matched by
// name, so undoing a reorder or a rename does not mix columns up.

const LIMIT = 200 // steps kept
const PAUSE = 500 // ms without change that end a step

export class History {
  past = $state([])
  future = $state([])
  #pending = $state(null) // snapshot not recorded yet (editing in progress)
  #current = null // the snapshot the document is at
  #key = null // which document: another one starts a new history
  #timer = null

  get canUndo() { return this.past.length > 0 || this.#pending !== null }
  get canRedo() { return this.future.length > 0 && this.#pending === null }

  // track is called with the document's snapshot whenever it changes.
  track(key, snapshot) {
    if (key !== this.#key) {
      this.#key = key
      this.#current = snapshot
      this.past = []
      this.future = []
      this.#cancel()
      return
    }
    if (snapshot === this.#current) {
      this.#cancel()
      return
    }
    this.#pending = snapshot
    clearTimeout(this.#timer)
    this.#timer = setTimeout(() => this.flush(), PAUSE)
  }

  // flush records the change in progress as a step.
  flush() {
    clearTimeout(this.#timer)
    if (this.#pending === null) return
    this.past.push(this.#current)
    if (this.past.length > LIMIT) this.past.shift()
    this.#current = this.#pending
    this.#pending = null
    this.future = []
  }

  // undo and redo return the snapshot to restore, or null.
  undo() {
    this.flush()
    if (!this.past.length) return null
    this.future.push(this.#current)
    this.#current = this.past.pop()
    return this.#current
  }

  redo() {
    this.flush()
    if (!this.future.length) return null
    this.past.push(this.#current)
    this.#current = this.future.pop()
    return this.#current
  }

  #cancel() {
    clearTimeout(this.#timer)
    this.#pending = null
  }
}

const isObject = (v) => v !== null && typeof v === 'object' && !Array.isArray(v)
const isNamed = (v) => isObject(v) && typeof v.name === 'string'

// patch makes dst equal to src (plain data), changing dst in place.
export function patch(dst, src) {
  for (const k of Object.keys(dst)) if (!(k in src)) delete dst[k]
  for (const [k, v] of Object.entries(src)) {
    const cur = dst[k]
    if (Array.isArray(v) && Array.isArray(cur)) patchList(cur, v)
    else if (isObject(v) && isObject(cur)) patch(cur, v)
    else if (cur !== v) dst[k] = v
  }
}

function patchList(dst, src) {
  let reuse
  if (src.every(isNamed) && dst.every(isNamed)) {
    // Same name first, then the others in order (a renamed item).
    const pool = [...dst]
    const take = (i) => (i < 0 ? undefined : pool.splice(i, 1)[0])
    reuse = src.map((s) => take(pool.findIndex((d) => d.name === s.name)))
    reuse = reuse.map((d) => d ?? take(pool.length ? 0 : -1))
  } else {
    reuse = dst.slice(0, src.length)
  }
  const items = src.map((s, i) => {
    const d = reuse[i]
    if (isObject(d) && isObject(s)) {
      patch(d, s)
      return d
    }
    return s
  })
  dst.splice(0, dst.length, ...items)
}
