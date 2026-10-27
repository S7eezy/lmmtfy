// Reordering table rows by dragging their grip, for the mapping and target
// editors. Pointer events behave the same in every web view, and do not
// interfere with dropping files on the window. The list scrolls by itself
// near its edges while dragging; arrow keys on a focused grip move its row.
//
// Options: list() and body() return the scrolling element and the <tbody>,
// count() the number of rows, move(from, to) moves a row, select(i) selects
// one, and off() tells when reordering is disabled (a filter is set).
export class Reorder {
  drag = $state(null) // { from, to, x, y }
  #frame = 0
  #lastTime = 0

  constructor(opts) {
    this.o = opts
  }

  start = (e, i) => {
    if (this.o.off() || e.button !== 0) return
    e.preventDefault()
    try {
      e.currentTarget.setPointerCapture(e.pointerId) // keep receiving moves outside the grip
    } catch {}
    this.drag = { from: i, to: i, x: e.clientX, y: e.clientY }
    this.o.select(i)
    this.#lastTime = 0
    this.#frame = requestAnimationFrame(this.#autoScroll)
  }

  move = (e) => {
    if (!this.drag) return
    this.drag.x = e.clientX
    this.drag.y = e.clientY
    this.#updateTarget()
  }

  end = () => {
    if (!this.drag) return
    const { from, to } = this.drag
    this.drag = null
    cancelAnimationFrame(this.#frame)
    if (from !== to) {
      this.o.move(from, to)
      this.o.select(to)
    }
  }

  cancel = (e) => {
    if (e.key === 'Escape' && this.drag) {
      this.drag = null
      cancelAnimationFrame(this.#frame)
    }
  }

  key = (e, i) => {
    const d = e.key === 'ArrowUp' ? -1 : e.key === 'ArrowDown' ? 1 : 0
    if (!d || this.o.off()) return
    e.preventDefault()
    const to = Math.max(0, Math.min(this.o.count() - 1, i + d))
    this.o.move(i, to)
    this.o.select(to)
    requestAnimationFrame(() => this.o.body()?.rows[to]?.querySelector('.grip')?.focus())
  }

  dropClass = (i) => {
    const d = this.drag
    if (!d || d.from === d.to || i !== d.to) return ''
    return d.to > d.from ? 'drop-below' : 'drop-above'
  }

  // Rows all have the same height: the row under the pointer is found by
  // arithmetic, not by measuring hundreds of rows.
  #updateTarget() {
    const first = this.o.body()?.rows[0]
    if (!first) return
    const top = first.getBoundingClientRect().top
    const h = first.offsetHeight || 1
    this.drag.to = Math.max(0, Math.min(this.o.count() - 1, Math.floor((this.drag.y - top) / h)))
  }

  // Scrolls while the pointer is near the top or bottom edge, faster the
  // closer it gets (up to 1500 px/s). Speed depends on elapsed time, not on
  // the frame rate, so slow machines scroll as fast as quick ones.
  #autoScroll = (now = performance.now()) => {
    const list = this.o.list()
    if (!this.drag || !list) return
    const dt = this.#lastTime ? Math.min(now - this.#lastTime, 100) / 1000 : 0
    this.#lastTime = now
    const r = list.getBoundingClientRect()
    const edge = 56
    const y = this.drag.y
    const depth = y < r.top + edge ? -(r.top + edge - y) : y > r.bottom - edge ? y - (r.bottom - edge) : 0
    if (depth) {
      list.scrollTop += Math.sign(depth) * Math.min(1, Math.abs(depth) / edge) * 1500 * dt
      this.#updateTarget()
    }
    this.#frame = requestAnimationFrame(this.#autoScroll)
  }
}
