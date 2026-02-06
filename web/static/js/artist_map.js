(() => {
  const mapEl = document.getElementById("map");
  const dataEl = document.getElementById("artist_locations_json");
  if (!mapEl || !dataEl) return;

  let locations;
  try {
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

  const map = window.L.map(mapEl, { scrollWheelZoom: false });

  window.L.tileLayer("https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png", {
    maxZoom: 19,
    attribution:
      '&copy; <a href="https://www.openstreetmap.org/copyright" target="_blank" rel="noopener noreferrer">OpenStreetMap</a> contributors',
  }).addTo(map);

  const bounds = [];

  for (const loc of locations) {
    const lat = Number(loc.lat);
    const lng = Number(loc.lng);
    if (!Number.isFinite(lat) || !Number.isFinite(lng)) continue;

    const name = String(loc.name || "").trim() || "Unknown location";
    const dates = Array.isArray(loc.dates) ? loc.dates : [];

    const popup = document.createElement("div");
    const title = document.createElement("div");
    title.textContent = name;
    title.style.fontWeight = "600";
    popup.appendChild(title);

    if (dates.length > 0) {
      const ul = document.createElement("ul");
      ul.style.margin = "6px 0 0";
      ul.style.paddingLeft = "18px";
      for (const d of dates) {
        const li = document.createElement("li");
        li.textContent = String(d);
        ul.appendChild(li);
      }
      popup.appendChild(ul);
    }

    window.L.marker([lat, lng]).addTo(map).bindPopup(popup);
    bounds.push([lat, lng]);
  }

  if (bounds.length === 0) {
    map.setView([0, 0], 2);
    return;
  }

  map.fitBounds(bounds, { padding: [24, 24] });

  function showMessage(message) {
    mapEl.innerHTML = "";
    const wrap = document.createElement("div");
    wrap.textContent = message;
    wrap.style.height = "100%";
    wrap.style.width = "100%";
    wrap.style.display = "flex";
    wrap.style.alignItems = "center";
    wrap.style.justifyContent = "center";
    wrap.style.fontSize = "12px";
    wrap.style.color = "rgb(148 163 184)"; // slate-400-ish
    mapEl.appendChild(wrap);
  }
})();
