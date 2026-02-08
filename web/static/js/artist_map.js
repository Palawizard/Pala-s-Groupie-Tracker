(() => { // IIFE to avoid leaking globals
  // Artist map rendering for Groupie concert locations (Leaflet + OSM tiles)
  const mapEl = document.getElementById("map");
  const dataEl = document.getElementById("artist_locations_json");
  if (!mapEl || !dataEl) return;

  let locations;
  try {
    // Locations come from a <script type="application/json"> tag in the template
    locations = JSON.parse(dataEl.textContent || "[]");
  } catch {
    return;
  }

  if (!Array.isArray(locations) || locations.length === 0) {
    showMessage("No map data available.");
    return;
  }

  if (typeof window.L === "undefined" || typeof window.L.map !== "function") {
    showMessage("Map library failed to load.");
    return;
  }

  // Disable scroll wheel zoom to avoid trapping the page scroll
  const map = window.L.map(mapEl, { scrollWheelZoom: false });

  window.L.tileLayer("https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png", {
    maxZoom: 19,
    attribution:
      '&copy; <a href="https://www.openstreetmap.org/copyright" target="_blank" rel="noopener noreferrer">OpenStreetMap</a> contributors',
  }).addTo(map);

  function markerKey(lat, lng) {
    // Keep stable across scripts and avoid float noise.
    return Number(lat).toFixed(5) + "," + Number(lng).toFixed(5);
  }

  const bounds = [];
  const state = window.__groupieTracker || (window.__groupieTracker = {});
  state.locationMarkersByKey = state.locationMarkersByKey || {};

  for (const loc of locations) {
    const lat = Number(loc.lat);
    const lng = Number(loc.lng);
    if (!Number.isFinite(lat) || !Number.isFinite(lng)) continue;

    const name = String(loc.name || "").trim() || "Unknown location";
    const dates = Array.isArray(loc.dates) ? loc.dates : [];

    // Build popup content with DOM nodes to avoid HTML injection
    const popup = document.createElement("div");
    const title = document.createElement("div");
    title.textContent = name;
    title.className = "font-semibold";
    popup.appendChild(title);

    if (dates.length > 0) {
      const ul = document.createElement("ul");
      ul.className = "mt-1 list-disc pl-5 text-xs";
      for (const d of dates) {
        const li = document.createElement("li");
        li.textContent = String(d);
        ul.appendChild(li);
      }
      popup.appendChild(ul);
    }

    const marker = window.L.marker([lat, lng]).addTo(map).bindPopup(popup);
    state.locationMarkersByKey[markerKey(lat, lng)] = marker;
    // Use bounds to auto-fit the map view to all markers
    bounds.push([lat, lng]);
  }

  if (bounds.length === 0) {
    // No valid points, show a generic world view
    map.setView([0, 0], 2);
    return;
  }

  map.fitBounds(bounds, { padding: [24, 24] });

  // Expose a small hook so other scripts (geolocation/viz) can reuse the map instance.
  state.leafletMap = map;
  state.concertBounds = bounds.slice();

  // showMessage replaces the map with a small centered message
  function showMessage(message) {
    mapEl.innerHTML = "";
    const wrap = document.createElement("div");
    wrap.textContent = message;
    wrap.className = "h-full w-full flex items-center justify-center text-xs text-slate-400";
    mapEl.appendChild(wrap);
  }
})();
