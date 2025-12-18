(function () {
    var form = document.getElementById("artist-filters");
    var list = document.getElementById("artist-list");
    if (!form || !list) return;

    var inputs = form.querySelectorAll("input, select");
    var timeoutId;

    var yearMin = document.getElementById("year_min");
    var yearMax = document.getElementById("year_max");
    var membersMin = document.getElementById("members_min");
    var membersMax = document.getElementById("members_max");

    var yearMinOut = document.getElementById("year_min_out");
    var yearMaxOut = document.getElementById("year_max_out");
    var membersMinOut = document.getElementById("members_min_out");
    var membersMaxOut = document.getElementById("members_max_out");

    var yearFill = document.getElementById("year_fill");
    var membersFill = document.getElementById("members_fill");

    function getCurrentSource() {
        try {
            var url = new URL(window.location.href);
            var s = (url.searchParams.get("source") || "").trim().toLowerCase();
            if (s === "spotify" || s === "deezer" || s === "groupie") return s;
        } catch (e) {
        }

        var sourceInput = document.getElementById("source");
        var v = sourceInput ? String(sourceInput.value || "").trim().toLowerCase() : "";
        if (v === "spotify" || v === "deezer") return v;
        return "groupie";
    }

    function toInt(v) {
        var n = parseInt(v, 10);
        return Number.isFinite(n) ? n : 0;
    }

    function updateFill(minEl, maxEl, fillEl) {
        if (!minEl || !maxEl || !fillEl) return;

        var minBound = toInt(minEl.min);
        var maxBound = toInt(minEl.max);
        var minV = toInt(minEl.value);
        var maxV = toInt(maxEl.value);

        var span = maxBound - minBound;
        if (span <= 0) {
            fillEl.style.left = "0%";
            fillEl.style.width = "100%";
            return;
        }

        var left = ((minV - minBound) / span) * 100;
        var right = ((maxV - minBound) / span) * 100;

        if (left < 0) left = 0;
        if (right > 100) right = 100;
        if (left > right) left = right;

        fillEl.style.left = left + "%";
        fillEl.style.width = (right - left) + "%";
    }

    function updateZIndex(minEl, maxEl) {
        if (!minEl || !maxEl) return;

        var minV = toInt(minEl.value);
        var maxV = toInt(maxEl.value);

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

        var minV = toInt(minEl.value);
        var maxV = toInt(maxEl.value);

        if (minV > maxV) {
            var active = document.activeElement;
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

        var params = new URLSearchParams();
        var formData = new FormData(form);
        formData.forEach(function (value, key) {
            if (value !== "") {
                params.append(key, value.toString());
            }
        });

        params.set("source", getCurrentSource());
        return params.toString();
    }

    function fetchArtists() {
        var query = buildQuery();
        var url = "/artists/ajax?" + query;

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
        var type = input.tagName.toLowerCase() === "select" ? "select" : input.type;
        var eventName = (type === "text" || type === "number" || type === "range") ? "input" : "change";
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
