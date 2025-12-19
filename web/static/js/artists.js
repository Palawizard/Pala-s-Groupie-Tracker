(function () {
    const form = document.getElementById("artist-filters");
    const list = document.getElementById("artist-list");
    if (!form || !list) return;

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

    function getCurrentSource() {
        try {
            const u = new URL(window.location.href);
            const s = (u.searchParams.get("source") || "").trim().toLowerCase();
            if (s === "spotify" || s === "deezer" || s === "apple" || s === "groupie") return s;
        } catch (e) {
        }

        const sourceInput = document.getElementById("source");
        const v = sourceInput ? String(sourceInput.value || "").trim().toLowerCase() : "";
        if (v === "spotify" || v === "deezer" || v === "apple") return v;
        return "groupie";
    }

    function toInt(v) {
        const n = parseInt(v, 10);
        return Number.isFinite(n) ? n : 0;
    }

    function updateFill(minEl, maxEl, fillEl) {
        if (!minEl || !maxEl || !fillEl) return;

        const minBound = toInt(minEl.min);
        const maxBound = toInt(minEl.max);
        const minV = toInt(minEl.value);
        const maxV = toInt(maxEl.value);

        const span = maxBound - minBound;
        if (span <= 0) {
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

    function clampPair(minEl, maxEl, minOutEl, maxOutEl, fillEl) {
        if (!minEl || !maxEl) return;

        let minV = toInt(minEl.value);
        let maxV = toInt(maxEl.value);

        if (minV > maxV) {
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

    function normalizeRanges() {
        if (getCurrentSource() !== "groupie") return;
        clampPair(yearMin, yearMax, yearMinOut, yearMaxOut, yearFill);
        clampPair(membersMin, membersMax, membersMinOut, membersMaxOut, membersFill);
    }

    function buildQuery() {
        normalizeRanges();

        const params = new URLSearchParams();
        const formData = new FormData(form);

        formData.forEach(function (value, key) {
            if (value !== "") {
                params.append(key, value.toString());
            }
        });

        params.set("source", getCurrentSource());
        return params.toString();
    }

    function fetchArtists() {
        const query = buildQuery();
        const url = "/artists/ajax?" + query;

        fetch(url, {
            headers: {
                "Accept": "text/html"
            }
        })
            .then(function (res) {
                if (!res.ok) {
                    throw new Error("request failed " + res.status);
                }
                return res.text();
            })
            .then(function (html) {
                list.innerHTML = html;
            })
            .catch(function () {
                list.innerHTML = '<p class="text-sm text-slate-400 col-span-full">Failed to load filtered artists.</p>';
            });
    }

    function scheduleFetch() {
        normalizeRanges();
        if (timeoutId) {
            window.clearTimeout(timeoutId);
        }
        timeoutId = window.setTimeout(fetchArtists, 250);
    }

    inputs.forEach(function (input) {
        const type = input.tagName.toLowerCase() === "select" ? "select" : input.type;
        const eventName = (type === "text" || type === "number" || type === "range") ? "input" : "change";
        input.addEventListener(eventName, scheduleFetch);
    });

    form.addEventListener("submit", function (event) {
        event.preventDefault();
        if (timeoutId) {
            window.clearTimeout(timeoutId);
        }
        fetchArtists();
    });

    normalizeRanges();
})();
