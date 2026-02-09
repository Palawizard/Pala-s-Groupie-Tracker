(() => { // IIFE to avoid leaking globals
  const dataEl = document.getElementById("artist_locations_json");
  const timelineEl = document.getElementById("concert_timeline");

  let locations = [];
  if (dataEl) {
    try {
      locations = JSON.parse(dataEl.textContent || "[]");
    } catch {
      locations = [];
    }
  }

  renderTimeline(locations);
  bindMapControls();

  function bindMapControls() {
    const fitBtn = document.getElementById("map_fit_btn");
    const geoBtn = document.getElementById("map_geolocate_btn");
    const statusEl = document.getElementById("map_geolocate_status");

    const state = window.__groupieTracker || {};
    const map = state.leafletMap;
    const L = window.L;

    if (fitBtn && map && Array.isArray(state.concertBounds) && state.concertBounds.length > 0) {
      fitBtn.addEventListener("click", () => {
        map.fitBounds(state.concertBounds, { padding: [24, 24] });
      });
    } else if (fitBtn) {
      fitBtn.disabled = true;
      fitBtn.classList.add("opacity-50", "cursor-not-allowed");
    }

    if (!geoBtn) return;
    if (!map || !L) {
      geoBtn.disabled = true;
      geoBtn.classList.add("opacity-50", "cursor-not-allowed");
      return;
    }

    if (!navigator.geolocation || typeof navigator.geolocation.getCurrentPosition !== "function") {
      geoBtn.disabled = true;
      geoBtn.classList.add("opacity-50", "cursor-not-allowed");
      if (statusEl) statusEl.textContent = "Geolocation not supported.";
      return;
    }

    geoBtn.addEventListener("click", () => {
      geoBtn.disabled = true;
      geoBtn.classList.add("opacity-50");
      if (statusEl) statusEl.textContent = "Locating...";

      navigator.geolocation.getCurrentPosition(
        (pos) => {
          const lat = Number(pos.coords.latitude);
          const lng = Number(pos.coords.longitude);
          if (!Number.isFinite(lat) || !Number.isFinite(lng)) {
            onError();
            return;
          }

          if (state.userMarker) {
            try { map.removeLayer(state.userMarker); } catch { /* ignore */ }
            state.userMarker = null;
          }

          const marker = L.circleMarker([lat, lng], {
            radius: 8,
            color: "#10b981",
            weight: 2,
            fillColor: "#10b981",
            fillOpacity: 0.35,
          }).addTo(map);

          marker.bindPopup("You are here.").openPopup();
          state.userMarker = marker;

          const bounds = Array.isArray(state.concertBounds) ? state.concertBounds.slice() : [];
          bounds.push([lat, lng]);
          if (bounds.length > 0) {
            map.fitBounds(bounds, { padding: [24, 24] });
          } else {
            map.setView([lat, lng], 10);
          }

          if (statusEl) statusEl.textContent = "";
          geoBtn.disabled = false;
          geoBtn.classList.remove("opacity-50");
        },
        onError,
        { enableHighAccuracy: false, timeout: 8000, maximumAge: 30000 }
      );

      function onError() {
        if (statusEl) statusEl.textContent = "Unable to get your position.";
        geoBtn.disabled = false;
        geoBtn.classList.remove("opacity-50");
      }
    });
  }

  // Timeline: each dot is a unique day with at least one concert date entry.
  function renderTimeline(locs) {
    if (!timelineEl) return;

    function markerKey(lat, lng) {
      return Number(lat).toFixed(5) + "," + Number(lng).toFixed(5);
    }

    const dayCounts = new Map(); // YYYY-MM-DD -> number of occurrences across locations
    const uniqueDays = new Set(); // YYYY-MM-DD
    const concertsByDay = new Map(); // YYYY-MM-DD -> Array<{name, lat, lng, count}>
    let rawDateEntries = 0;
    let totalLocations = 0;
    let minDate = null;
    let maxDate = null;

    for (const loc of Array.isArray(locs) ? locs : []) {
      if (loc && (loc.lat || loc.lng || loc.name || loc.dates)) totalLocations += 1;
      const dates = Array.isArray(loc && loc.dates) ? loc.dates : [];
      const lat = Number(loc && loc.lat);
      const lng = Number(loc && loc.lng);
      const locName = String((loc && loc.name) || "").trim() || "Unknown location";

      for (const d of dates) {
        const parsed = parseConcertDate(String(d || ""));
        if (!parsed) continue;

        rawDateEntries += 1;
        uniqueDays.add(parsed.iso);
        dayCounts.set(parsed.iso, (dayCounts.get(parsed.iso) || 0) + 1);

        if (Number.isFinite(lat) && Number.isFinite(lng)) {
          const key = markerKey(lat, lng) + "|" + locName;
          const list = concertsByDay.get(parsed.iso) || [];
          let found = null;
          for (const it of list) {
            if (it._k === key) {
              found = it;
              break;
            }
          }
          if (found) {
            found.count += 1;
          } else {
            // Private key used only for dedupe.
            list.push({ name: locName, lat, lng, count: 1, _k: key });
          }
          concertsByDay.set(parsed.iso, list);
        }

        if (!minDate || parsed.date < minDate) minDate = parsed.date;
        if (!maxDate || parsed.date > maxDate) maxDate = parsed.date;
      }
    }

    const activeDays = Array.from(uniqueDays).sort();
    if (activeDays.length === 0 || !minDate || !maxDate) {
      timelineEl.textContent = "No concert date data available.";
      return;
    }

    const w = 820;
    const h = 150;
    const padX = 18;
    const padTop = 26;
    const padBottom = 34;
    const plotH = h - padTop - padBottom;
    const baseY = padTop + Math.floor(plotH / 2);
    const lineLeft = padX;
    const lineRight = w - padX;

    const minT = minDate.getTime();
    const maxT = maxDate.getTime();
    const span = Math.max(1, maxT - minT);
    const width = lineRight - lineLeft;
    const xForTime = (t) => lineLeft + Math.round((width * (t - minT)) / span);

    const svgNS = "http://www.w3.org/2000/svg";
    const svg = document.createElementNS(svgNS, "svg");
    svg.setAttribute("viewBox", `0 0 ${w} ${h}`);
    svg.setAttribute("role", "img");
    svg.setAttribute("aria-label", "Concert day timeline");
    svg.className = "w-full h-28";

    const defs = document.createElementNS(svgNS, "defs");
    const glow = document.createElementNS(svgNS, "filter");
    glow.setAttribute("id", "gt_glow");
    glow.setAttribute("x", "-50%");
    glow.setAttribute("y", "-50%");
    glow.setAttribute("width", "200%");
    glow.setAttribute("height", "200%");
    const blur = document.createElementNS(svgNS, "feGaussianBlur");
    blur.setAttribute("stdDeviation", "1.0");
    blur.setAttribute("result", "blur");
    glow.appendChild(blur);
    const merge = document.createElementNS(svgNS, "feMerge");
    const mn1 = document.createElementNS(svgNS, "feMergeNode");
    mn1.setAttribute("in", "blur");
    const mn2 = document.createElementNS(svgNS, "feMergeNode");
    mn2.setAttribute("in", "SourceGraphic");
    merge.appendChild(mn1);
    merge.appendChild(mn2);
    glow.appendChild(merge);
    defs.appendChild(glow);
    svg.appendChild(defs);

    // Baseline
    const baseLine = document.createElementNS(svgNS, "line");
    baseLine.setAttribute("x1", String(lineLeft));
    baseLine.setAttribute("x2", String(lineRight));
    baseLine.setAttribute("y1", String(baseY));
    baseLine.setAttribute("y2", String(baseY));
    baseLine.setAttribute("stroke", "#334155"); // slate-700
    baseLine.setAttribute("stroke-width", "2");
    baseLine.setAttribute("stroke-linecap", "round");
    svg.appendChild(baseLine);

    // Range labels
    const leftLabel = document.createElementNS(svgNS, "text");
    leftLabel.setAttribute("x", String(lineLeft));
    leftLabel.setAttribute("y", "16");
    leftLabel.setAttribute("text-anchor", "start");
    leftLabel.setAttribute("fill", "#94a3b8");
    leftLabel.setAttribute("font-size", "10");
    leftLabel.textContent = formatISODate(minDate);
    svg.appendChild(leftLabel);

    const rightLabel = document.createElementNS(svgNS, "text");
    rightLabel.setAttribute("x", String(lineRight));
    rightLabel.setAttribute("y", "16");
    rightLabel.setAttribute("text-anchor", "end");
    rightLabel.setAttribute("fill", "#94a3b8");
    rightLabel.setAttribute("font-size", "10");
    rightLabel.textContent = formatISODate(maxDate);
    svg.appendChild(rightLabel);

    // Month ticks
    const monthStarts = monthsBetweenUTC(minDate, maxDate);
    const labelEvery = monthStarts.length <= 8 ? 1 : monthStarts.length <= 18 ? 2 : 3;
    for (let i = 0; i < monthStarts.length; i++) {
      const dt = monthStarts[i];
      const x = xForTime(dt.getTime());

      const tick = document.createElementNS(svgNS, "line");
      tick.setAttribute("x1", String(x));
      tick.setAttribute("x2", String(x));
      tick.setAttribute("y1", String(baseY - 10));
      tick.setAttribute("y2", String(baseY + 10));
      tick.setAttribute("stroke", "#1f2937");
      tick.setAttribute("stroke-opacity", "0.6");
      tick.setAttribute("stroke-width", "1");
      svg.appendChild(tick);

      if (i === 0 || i === monthStarts.length - 1 || i % labelEvery === 0) {
        const text = document.createElementNS(svgNS, "text");
        text.setAttribute("x", String(x));
        text.setAttribute("y", String(h - 12));
        text.setAttribute("text-anchor", "middle");
        text.setAttribute("fill", "#94a3b8");
        text.setAttribute("font-size", "10");
        text.textContent = formatMonthKey(dt);
        svg.appendChild(text);
      }
    }

    // Dots: keep them on the baseline (timeline). If multiple dots land on the same pixel X,
    // spread them horizontally a bit so they remain individually hoverable.
    const byX = new Map(); // number -> Array<{iso: string, c: number}>

    for (const iso of activeDays) {
      const dt = isoToUTCDate(iso);
      if (!dt) continue;
      const x = xForTime(dt.getTime());
      const c = dayCounts.get(iso) || 1;

      const arr = byX.get(x) || [];
      arr.push({ iso, c });
      byX.set(x, arr);
    }

    const xs = Array.from(byX.keys()).sort((a, b) => a - b);
    const dotByISO = new Map();
    let selectedISO = "";

    for (const x of xs) {
      const items = byX.get(x) || [];
      items.sort((a, b) => a.iso.localeCompare(b.iso));

      const n = items.length;
      const step = n > 10 ? 2 : n > 6 ? 3 : 4; // px
      const start = x - Math.floor((n - 1) / 2) * step;

      for (let i = 0; i < n; i++) {
        const it = items[i];
        let xAdj = start + i * step;
        if (xAdj < lineLeft) xAdj = lineLeft;
        if (xAdj > lineRight) xAdj = lineRight;

        const r = 3 + Math.min(3, Math.log2(it.c + 1));

        const dot = document.createElementNS(svgNS, "circle");
        dot.setAttribute("cx", String(xAdj));
        dot.setAttribute("cy", String(baseY));
        dot.setAttribute("r", String(r));
        dot.setAttribute("fill", "#34d399");
        dot.setAttribute("fill-opacity", it.c > 1 ? "0.95" : "0.75");
        dot.setAttribute("stroke", "#0b1220");
        dot.setAttribute("stroke-opacity", "0.9");
        dot.setAttribute("stroke-width", "1.5");
        dot.setAttribute("filter", "url(#gt_glow)");
        dot.style.cursor = "pointer";
        dot.setAttribute("tabindex", "0");
        dot.setAttribute("role", "button");
        dot.setAttribute("aria-label", `Concerts on ${it.iso}`);

        const t = document.createElementNS(svgNS, "title");
        t.textContent = `${it.iso}: ${it.c} concert(s)`;
        dot.appendChild(t);

        // Bigger invisible hit area for easier clicking.
        const hit = document.createElementNS(svgNS, "circle");
        hit.setAttribute("cx", String(xAdj));
        hit.setAttribute("cy", String(baseY));
        hit.setAttribute("r", String(Math.max(10, r + 7)));
        hit.setAttribute("fill", "transparent");
        hit.style.cursor = "pointer";

        const onActivate = () => selectDay(it.iso);
        dot.addEventListener("click", onActivate);
        hit.addEventListener("click", onActivate);
        dot.addEventListener("keydown", (ev) => {
          if (ev.key === "Enter" || ev.key === " ") {
            ev.preventDefault();
            onActivate();
          }
        });

        svg.appendChild(dot);
        svg.appendChild(hit);
        dotByISO.set(it.iso, dot);
      }
    }

    const header = document.createElement("div");
    header.className = "flex flex-wrap items-center justify-between gap-2 pb-2";
    const title = document.createElement("div");
    title.className = "text-[11px] text-slate-600 dark:text-slate-400";
    title.textContent = `${formatISODate(minDate)} to ${formatISODate(maxDate)} | ${activeDays.length} active day(s) | ${rawDateEntries} date entr${rawDateEntries === 1 ? "y" : "ies"} | ${totalLocations} location(s)`;
    header.appendChild(title);

    const info = document.createElement("div");
    info.className = "mt-3 rounded-xl border border-slate-200 bg-white p-3 dark:border-slate-800 dark:bg-slate-950/40";
    info.setAttribute("aria-live", "polite");
    info.textContent = "Click a dot to see the concerts for that date.";

    timelineEl.innerHTML = "";
    timelineEl.appendChild(header);
    timelineEl.appendChild(svg);
    timelineEl.appendChild(info);

    function selectDay(iso) {
      if (!iso) return;

      if (selectedISO && dotByISO.get(selectedISO)) {
        const prev = dotByISO.get(selectedISO);
        prev.setAttribute("stroke", document.documentElement.classList.contains("dark") ? "#0b1220" : "#f1f5f9");
        prev.setAttribute("stroke-width", "1.5");
      }

      selectedISO = iso;
      const cur = dotByISO.get(iso);
      if (cur) {
        cur.setAttribute("stroke", document.documentElement.classList.contains("dark") ? "#e2e8f0" : "#0b1220");
        cur.setAttribute("stroke-width", "2.5");
      }

      const concerts = (concertsByDay.get(iso) || []).slice().sort((a, b) => a.name.localeCompare(b.name));
      const total = dayCounts.get(iso) || concerts.reduce((s, c) => s + (c.count || 1), 0);

      info.innerHTML = "";

      const top = document.createElement("div");
      top.className = "flex flex-wrap items-center justify-between gap-2";
      const h = document.createElement("div");
      h.className = "text-sm font-semibold text-slate-900 dark:text-slate-200";
      h.textContent = iso;
      const meta = document.createElement("div");
      meta.className = "text-xs text-slate-600 dark:text-slate-400";
      meta.textContent = `${total} concert(s)`;
      top.appendChild(h);
      top.appendChild(meta);
      info.appendChild(top);

      if (concerts.length === 0) {
        const p = document.createElement("p");
        p.className = "mt-2 text-xs text-slate-600 dark:text-slate-400";
        p.textContent = "No mappable location details for this day.";
        info.appendChild(p);
        return;
      }

      const ul = document.createElement("ul");
      ul.className = "mt-2 space-y-2";
      for (const c of concerts) {
        const li = document.createElement("li");
        li.className = "flex items-center justify-between gap-3 rounded-lg border border-slate-200 bg-slate-50 px-3 py-2 dark:border-slate-800 dark:bg-slate-950/40";

        const left = document.createElement("div");
        left.className = "min-w-0";
        const name = document.createElement("div");
        name.className = "truncate text-xs text-slate-900 dark:text-slate-200";
        name.textContent = c.name;
        const sub = document.createElement("div");
        sub.className = "text-[11px] text-slate-600 dark:text-slate-400";
        sub.textContent = c.count > 1 ? `${c.count} entries` : "1 entry";
        left.appendChild(name);
        left.appendChild(sub);

        const btn = document.createElement("button");
        btn.type = "button";
        btn.className = "shrink-0 inline-flex items-center rounded-full border border-slate-300 px-3 py-1 text-xs font-medium text-slate-700 hover:bg-slate-100 transition-colors dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-800/90";
        btn.textContent = "Show on map";
        btn.addEventListener("click", () => focusMap(c.lat, c.lng));

        li.appendChild(left);
        li.appendChild(btn);
        ul.appendChild(li);
      }
      info.appendChild(ul);
    }

    function focusMap(lat, lng) {
      const state = window.__groupieTracker || {};
      const map = state.leafletMap;
      const L = window.L;
      if (!map || !L) return;

      const key = markerKey(lat, lng);
      const marker = state.locationMarkersByKey ? state.locationMarkersByKey[key] : null;
      if (marker && typeof marker.openPopup === "function") {
        map.setView([lat, lng], Math.max(map.getZoom(), 5));
        marker.openPopup();
        return;
      }

      // Fallback: show a temporary marker if we couldn't match one.
      if (state.timelineTempMarker) {
        try { map.removeLayer(state.timelineTempMarker); } catch { /* ignore */ }
        state.timelineTempMarker = null;
      }
      const m = L.circleMarker([lat, lng], {
        radius: 7,
        color: "#38bdf8",
        weight: 2,
        fillColor: "#38bdf8",
        fillOpacity: 0.25,
      }).addTo(map);
      state.timelineTempMarker = m;
      map.setView([lat, lng], Math.max(map.getZoom(), 5));
    }
  }

  function parseConcertDate(s) {
    const v = String(s || "").trim();
    if (!v) return null;

    // YYYY-MM-DD
    let m = /^(\d{4})[-/](\d{2})[-/](\d{2})$/.exec(v);
    if (m) {
      const y = parseInt(m[1], 10);
      const mo = parseInt(m[2], 10);
      const d = parseInt(m[3], 10);
      if (!Number.isFinite(y) || !Number.isFinite(mo) || !Number.isFinite(d)) return null;
      const iso = `${m[1]}-${m[2]}-${m[3]}`;
      return { iso, date: new Date(Date.UTC(y, mo - 1, d)) };
    }

    // DD-MM-YYYY
    m = /^(\d{2})[-/](\d{2})[-/](\d{4})$/.exec(v);
    if (m) {
      const y = parseInt(m[3], 10);
      const mo = parseInt(m[2], 10);
      const d = parseInt(m[1], 10);
      if (!Number.isFinite(y) || !Number.isFinite(mo) || !Number.isFinite(d)) return null;
      const iso = `${m[3]}-${m[2]}-${m[1]}`;
      return { iso, date: new Date(Date.UTC(y, mo - 1, d)) };
    }

    return null;
  }

  function isoToUTCDate(iso) {
    const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(String(iso || "").trim());
    if (!m) return null;
    const y = parseInt(m[1], 10);
    const mo = parseInt(m[2], 10);
    const d = parseInt(m[3], 10);
    if (!Number.isFinite(y) || !Number.isFinite(mo) || !Number.isFinite(d)) return null;
    return new Date(Date.UTC(y, mo - 1, d));
  }

  function formatMonthKey(dt) {
    const d = dt instanceof Date ? dt : new Date(dt);
    if (!(d instanceof Date) || Number.isNaN(d.getTime())) return "";
    const y = d.getUTCFullYear();
    const mo = String(d.getUTCMonth() + 1).padStart(2, "0");
    return `${y}-${mo}`;
  }

  function monthsBetweenUTC(minD, maxD) {
    const min = minD instanceof Date ? minD : new Date(minD);
    const max = maxD instanceof Date ? maxD : new Date(maxD);
    if (!(min instanceof Date) || Number.isNaN(min.getTime())) return [];
    if (!(max instanceof Date) || Number.isNaN(max.getTime())) return [];

    const out = [];
    let y = min.getUTCFullYear();
    let m = min.getUTCMonth();
    const endY = max.getUTCFullYear();
    const endM = max.getUTCMonth();

    while (y < endY || (y === endY && m <= endM)) {
      out.push(new Date(Date.UTC(y, m, 1)));
      m += 1;
      if (m >= 12) {
        m = 0;
        y += 1;
      }
      if (out.length > 600) break;
    }
    return out;
  }

  function formatISODate(date) {
    const d = date instanceof Date ? date : new Date(date);
    if (!(d instanceof Date) || Number.isNaN(d.getTime())) return "";
    const y = d.getUTCFullYear();
    const mo = String(d.getUTCMonth() + 1).padStart(2, "0");
    const da = String(d.getUTCDate()).padStart(2, "0");
    return `${y}-${mo}-${da}`;
  }
})();
