(function () { // IIFE to avoid leaking globals
    // Live filters for the artists page, uses /artists/ajax to refresh the list
    const form = document.getElementById("artist-filters");
    const list = document.getElementById("artist-list");
    if (!form || !list) return;

    const loadingEl = document.getElementById("artist_loading");
    const resultsEl = document.getElementById("artist_results");

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

    function setLoading(isLoading) {
        if (!loadingEl) return;
        if (isLoading) {
            loadingEl.classList.remove("hidden");
            list.setAttribute("aria-busy", "true");
        } else {
            loadingEl.classList.add("hidden");
            list.removeAttribute("aria-busy");
        }
    }

    function updateResultCount() {
        if (!resultsEl) return;
        const cards = list.querySelectorAll("a").length;
        resultsEl.textContent = String(cards) + " results";
    }

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

        setLoading(true);
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
                updateResultCount();
                setLoading(false);
            })
            .catch(function () { // errors are expected when APIs are down
                list.innerHTML = '<p class="text-sm text-slate-600 dark:text-slate-400 col-span-full">Failed to load filtered artists.</p>';
                updateResultCount();
                setLoading(false);
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
    updateResultCount();

    // Search suggestions (Groupie mode only)
    const searchInput = document.getElementById("q");
    const locationInput = document.getElementById("location");
    const suggestWrap = document.getElementById("search_suggest");
    const suggestList = document.getElementById("search_suggest_list");

    if (!searchInput || !suggestWrap || !suggestList) return;

    let suggestTimeoutId;
    let activeIndex = -1;
    let lastItems = [];

    function hideSuggestions() {
        suggestWrap.classList.add("hidden");
        suggestList.innerHTML = "";
        activeIndex = -1;
        lastItems = [];
    }

    function setActive(index) {
        const items = suggestList.querySelectorAll("[data-suggest-item]");
        items.forEach(function (el, i) {
            if (i === index) {
                el.classList.add("bg-slate-100", "dark:bg-slate-900/70");
                el.setAttribute("aria-selected", "true");
            } else {
                el.classList.remove("bg-slate-100", "dark:bg-slate-900/70");
                el.removeAttribute("aria-selected");
            }
        });
        activeIndex = index;
    }

    function applySuggestion(s) {
        if (!s) return;
        const target = String(s.target || "q");
        const value = String(s.value || "");

        if (target === "location" && locationInput) {
            locationInput.value = value;
            locationInput.focus();
        } else {
            searchInput.value = value;
            searchInput.focus();
        }

        hideSuggestions();
        scheduleFetch();
    }

    function renderSuggestions(items) {
        suggestList.innerHTML = "";
        lastItems = Array.isArray(items) ? items : [];
        activeIndex = -1;

        if (!Array.isArray(items) || items.length === 0) {
            hideSuggestions();
            return;
        }

        for (const s of items) {
            const li = document.createElement("li");
            li.setAttribute("data-suggest-item", "1");
            li.className = "flex items-center justify-between gap-2 px-3 py-2 text-sm text-slate-800 hover:bg-slate-100 cursor-pointer dark:text-slate-200 dark:hover:bg-slate-900/70";

            const left = document.createElement("div");
            left.className = "min-w-0";

            const label = document.createElement("div");
            label.className = "truncate";
            label.textContent = String(s.label || s.value || "");

            left.appendChild(label);

            const badge = document.createElement("span");
            badge.className = "shrink-0 rounded-full border border-slate-300 px-2 py-0.5 text-[11px] text-slate-600 dark:border-slate-700 dark:text-slate-300";
            badge.textContent = String(s.type || "suggestion");

            li.appendChild(left);
            li.appendChild(badge);

            li.addEventListener("mousedown", function (ev) { // mousedown prevents blur before click
                ev.preventDefault();
                applySuggestion(s);
            });

            suggestList.appendChild(li);
        }

        suggestWrap.classList.remove("hidden");
    }

    function fetchSuggestions() {
        if (getCurrentSource() !== "groupie") {
            hideSuggestions();
            return;
        }

        const q = String(searchInput.value || "").trim();
        if (q.length < 2) {
            hideSuggestions();
            return;
        }

        const basePath = getBasePath();
        const params = new URLSearchParams();
        params.set("source", "groupie");
        params.set("q", q);

        const url = (basePath || "") + "/artists/suggest?" + params.toString();
        fetch(url, {
            headers: {
                "Accept": "application/json"
            }
        })
            .then(function (res) {
                if (!res.ok) throw new Error("suggest failed " + res.status);
                return res.json();
            })
            .then(function (json) {
                renderSuggestions(json);
            })
            .catch(function () {
                hideSuggestions();
            });
    }

    function scheduleSuggest() {
        if (suggestTimeoutId) window.clearTimeout(suggestTimeoutId);
        suggestTimeoutId = window.setTimeout(fetchSuggestions, 120);
    }

    searchInput.addEventListener("input", scheduleSuggest);

    searchInput.addEventListener("keydown", function (ev) {
        if (suggestWrap.classList.contains("hidden")) return;
        const count = lastItems.length;
        if (count === 0) return;

        if (ev.key === "ArrowDown") {
            ev.preventDefault();
            const next = activeIndex < 0 ? 0 : Math.min(activeIndex + 1, count - 1);
            setActive(next);
            return;
        }
        if (ev.key === "ArrowUp") {
            ev.preventDefault();
            const next = activeIndex <= 0 ? 0 : activeIndex - 1;
            setActive(next);
            return;
        }
        if (ev.key === "Enter") {
            if (activeIndex >= 0 && activeIndex < count) {
                ev.preventDefault();
                applySuggestion(lastItems[activeIndex]);
            }
            return;
        }
        if (ev.key === "Escape") {
            ev.preventDefault();
            hideSuggestions();
        }
    });

    document.addEventListener("click", function (ev) {
        if (ev.target === searchInput) return;
        if (suggestWrap.contains(ev.target)) return;
        hideSuggestions();
    });
})();
