(function () { // IIFE to avoid leaking globals
    // Live filters for the artists page, uses /artists/ajax to refresh the list
    const form = document.getElementById("artist-filters");
    const list = document.getElementById("artist-list");
    if (!form || !list) return;

    function getBasePath() {
        const bp = document.body ? (document.body.getAttribute("data-base-path") || "") : "";
        return String(bp || "").replace(/\/+$/, "");
    }

    const inputs = form.querySelectorAll("input, select");
    let timeoutId;

    const yearMin = document.getElementById("year_min");
    const yearMax = document.getElementById("year_max");
    const membersMin = document.getElementById("members_min");
    const membersMax = document.getElementById("members_max");

    const yearMinOut = document.getElementById("year_min_out");
    const yearMaxOut = document.getElementById("year_max_out");
    const membersMinOut = document.getElementById("members_min_out");
    const membersMaxOut = document.getElementById("members_max_out");

    const yearFill = document.getElementById("year_fill");
    const membersFill = document.getElementById("members_fill");

    // getCurrentSource reads the current "source" mode from the URL or hidden input
    function getCurrentSource() {
        try {
            const u = new URL(window.location.href);
            const s = (u.searchParams.get("source") || "").trim().toLowerCase();
            if (s === "spotify" || s === "deezer" || s === "apple" || s === "groupie") return s;
        } catch (e) {
            // Ignore invalid URL parsing and fall back to the hidden input
        }

        const sourceInput = document.getElementById("source");
        const v = sourceInput ? String(sourceInput.value || "").trim().toLowerCase() : "";
        if (v === "spotify" || v === "deezer" || v === "apple") return v;
        return "groupie";
    }

    // toInt parses an int safely and returns 0 for invalid values
    function toInt(v) {
        const n = parseInt(v, 10);
        return Number.isFinite(n) ? n : 0;
    }

    // updateFill updates the highlighted range segment for the dual slider UI
    function updateFill(minEl, maxEl, fillEl) {
        if (!minEl || !maxEl || !fillEl) return;

        const minBound = toInt(minEl.min);
        const maxBound = toInt(minEl.max);
        const minV = toInt(minEl.value);
        const maxV = toInt(maxEl.value);

        const span = maxBound - minBound;
        if (span <= 0) {
            // Avoid division by zero if bounds are broken
            fillEl.style.left = "0%";
            fillEl.style.width = "100%";
            return;
        }

        let left = ((minV - minBound) / span) * 100;
        let right = ((maxV - minBound) / span) * 100;

        if (left < 0) left = 0;
        if (right > 100) right = 100;
        if (left > right) left = right;

        fillEl.style.left = left + "%";
        fillEl.style.width = (right - left) + "%";
    }

    // updateZIndex keeps the active thumb above the other when sliders overlap
    function updateZIndex(minEl, maxEl) {
        if (!minEl || !maxEl) return;

        const minV = toInt(minEl.value);
        const maxV = toInt(maxEl.value);

        if (minV >= maxV) {
            minEl.style.zIndex = "5";
            maxEl.style.zIndex = "4";
        } else {
            minEl.style.zIndex = "4";
            maxEl.style.zIndex = "5";
        }
    }

    // clampPair prevents min/max sliders from crossing and updates the UI outputs
    function clampPair(minEl, maxEl, minOutEl, maxOutEl, fillEl) {
        if (!minEl || !maxEl) return;

        let minV = toInt(minEl.value);
        let maxV = toInt(maxEl.value);

        if (minV > maxV) {
            // Keep whichever slider the user is moving as the "source of truth"
            const active = document.activeElement;
            if (active === minEl) {
                maxV = minV;
                maxEl.value = String(maxV);
            } else {
                minV = maxV;
                minEl.value = String(minV);
            }
        }

        if (minOutEl) minOutEl.textContent = String(toInt(minEl.value));
        if (maxOutEl) maxOutEl.textContent = String(toInt(maxEl.value));

        updateFill(minEl, maxEl, fillEl);
        updateZIndex(minEl, maxEl);
    }

    // normalizeRanges runs slider clamping only in Groupie mode
    function normalizeRanges() {
        if (getCurrentSource() !== "groupie") return;
        clampPair(yearMin, yearMax, yearMinOut, yearMaxOut, yearFill);
        clampPair(membersMin, membersMax, membersMinOut, membersMaxOut, membersFill);
    }

    // buildQuery serializes form state into a query string for /artists/ajax
    function buildQuery() {
        normalizeRanges();

        const params = new URLSearchParams();
        const formData = new FormData(form);

        // Only include non-empty values to keep URLs clean
        formData.forEach(function (value, key) { // callback runs for each field
            if (value !== "") {
                params.append(key, value.toString());
            }
        });

        // Always include the current source so the backend returns the right template section
        params.set("source", getCurrentSource());
        return params.toString();
    }

    // fetchArtists calls the ajax endpoint and replaces the list HTML
    function fetchArtists() {
        const query = buildQuery();
        const basePath = getBasePath();
        const url = (basePath || "") + "/artists/ajax?" + query;

        fetch(url, {
            headers: {
                "Accept": "text/html"
            }
        })
            .then(function (res) { // first stage checks HTTP status
                if (!res.ok) {
                    throw new Error("request failed " + res.status);
                }
                return res.text();
            })
            .then(function (html) { // second stage injects the HTML fragment
                list.innerHTML = html;
            })
            .catch(function () { // errors are expected when APIs are down
                list.innerHTML = '<p class="text-sm text-slate-400 col-span-full">Failed to load filtered artists.</p>';
            });
    }

    // scheduleFetch debounces requests while the user is typing or dragging sliders
    function scheduleFetch() {
        normalizeRanges();
        if (timeoutId) {
            window.clearTimeout(timeoutId);
        }
        timeoutId = window.setTimeout(fetchArtists, 250);
    }

    // Bind input listeners to all fields in the filter form
    inputs.forEach(function (input) { // callback runs for each input/select
        const type = input.tagName.toLowerCase() === "select" ? "select" : input.type;
        // Use "input" for responsive sliders/text, "change" for discrete controls
        const eventName = (type === "text" || type === "number" || type === "range") ? "input" : "change";
        input.addEventListener(eventName, scheduleFetch);
    });

    // Prevent full page reload and use the ajax update instead
    form.addEventListener("submit", function (event) { // submit handler
        event.preventDefault();
        if (timeoutId) {
            window.clearTimeout(timeoutId);
        }
        fetchArtists();
    });

    // Initialize slider UI on first load
    normalizeRanges();
})();
