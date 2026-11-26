// Hover for the line charts in the generated exhibits (pkg/internal/docs/chart.go):
// a crosshair that snaps to the nearest x, and a tooltip listing every series there.
// Built from text nodes only, never innerHTML.
type HoverData = {
  rows?: string[]
  series: { name: string; slot: number; values: (number | null)[] }[]
}

export function initCharts(root: ParentNode = document): void {
  root.querySelectorAll<HTMLElement>('.plot[data-chart]').forEach((plot) => {
    if (plot.dataset.bound) return
    plot.dataset.bound = 'true'

    const data = JSON.parse(plot.getAttribute('data-chart') ?? '{}') as HoverData
    const svg = plot.querySelector('svg') as SVGSVGElement
    const hit = plot.querySelector('.hit') as SVGRectElement
    const hair = plot.querySelector('.crosshair') as SVGLineElement
    const tip = plot.querySelector('.tooltip') as HTMLElement
    const x0 = Number(hit.dataset.x0)
    const x1 = Number(hit.dataset.x1)
    const n = Number(hit.dataset.n)

    const show = (evt: PointerEvent) => {
      const pt = svg.createSVGPoint()
      pt.x = evt.clientX
      pt.y = evt.clientY
      const p = pt.matrixTransform(svg.getScreenCTM()!.inverse())
      let i = n > 1 ? Math.round(((p.x - x0) / (x1 - x0)) * (n - 1)) : 0
      i = Math.max(0, Math.min(n - 1, i))
      const cx = n > 1 ? x0 + (i * (x1 - x0)) / (n - 1) : x0
      hair.setAttribute('x1', String(cx))
      hair.setAttribute('x2', String(cx))
      hair.style.visibility = 'visible'

      tip.textContent = ''
      const head = document.createElement('div')
      head.className = 'head'
      head.textContent = data.rows?.[i] ?? String(i + 1)
      tip.appendChild(head)

      for (const s of data.series) {
        const row = document.createElement('div')
        row.className = 'row'
        const key = document.createElement('i')
        key.style.background = `var(--series-${s.slot})`
        const name = document.createElement('span')
        name.textContent = s.name
        const val = document.createElement('b')
        val.textContent = s.values[i] == null ? '—' : s.values[i]!.toFixed(2)
        row.append(key, name, val)
        tip.appendChild(row)
      }

      tip.hidden = false
      const box = plot.getBoundingClientRect()
      let left = evt.clientX - box.left + 14
      if (left + tip.offsetWidth > box.width) left = evt.clientX - box.left - tip.offsetWidth - 14
      tip.style.left = `${left}px`
      tip.style.top = `${Math.max(0, evt.clientY - box.top - 10)}px`
    }

    const hide = () => {
      hair.style.visibility = 'hidden'
      tip.hidden = true
    }

    hit.addEventListener('pointermove', show)
    hit.addEventListener('pointerleave', hide)
  })
}
