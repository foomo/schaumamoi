document.querySelectorAll('.plot[data-chart]').forEach(function (plot) {
  var data = JSON.parse(plot.getAttribute('data-chart'));
  var svg = plot.querySelector('svg'), hit = plot.querySelector('.hit'), hair = plot.querySelector('.crosshair');
  var tip = plot.querySelector('.tooltip');
  var x0 = +hit.dataset.x0, x1 = +hit.dataset.x1, n = +hit.dataset.n;
  function show(evt) {
    var pt = svg.createSVGPoint(); pt.x = evt.clientX; pt.y = evt.clientY;
    var p = pt.matrixTransform(svg.getScreenCTM().inverse());
    var i = n > 1 ? Math.round((p.x - x0) / (x1 - x0) * (n - 1)) : 0;
    i = Math.max(0, Math.min(n - 1, i));
    var cx = n > 1 ? x0 + i * (x1 - x0) / (n - 1) : x0;
    hair.setAttribute('x1', cx); hair.setAttribute('x2', cx); hair.style.visibility = 'visible';
    tip.textContent = '';
    var head = document.createElement('div'); head.className = 'head';
    head.textContent = (data.rows && data.rows[i]) || String(i + 1); tip.appendChild(head);
    data.series.forEach(function (s) {
      var row = document.createElement('div'); row.className = 'row';
      var key = document.createElement('i'); key.style.background = 'var(--series-' + s.slot + ')';
      var name = document.createElement('span'); name.textContent = s.name;
      var val = document.createElement('b'); val.textContent = s.values[i] == null ? '—' : s.values[i].toFixed(2);
      row.appendChild(key); row.appendChild(name); row.appendChild(val); tip.appendChild(row);
    });
    tip.hidden = false;
    var box = plot.getBoundingClientRect(), left = evt.clientX - box.left + 14;
    if (left + tip.offsetWidth > box.width) left = evt.clientX - box.left - tip.offsetWidth - 14;
    tip.style.left = left + 'px'; tip.style.top = Math.max(0, evt.clientY - box.top - 10) + 'px';
  }
  function hide() { hair.style.visibility = 'hidden'; tip.hidden = true; }
  hit.addEventListener('pointermove', show);
  hit.addEventListener('pointerleave', hide);
});
