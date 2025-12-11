(function () {
    var form = document.getElementById("artist-filters");
    var list = document.getElementById("artist-list");
    var sourceInput = document.getElementById("source");
    if (!form || !list || !sourceInput) return;

    var currentSource = sourceInput.value === "spotify" ? "spotify" : "groupie";
    var inputs = form.querySelectorAll("input, select");
    var timeoutId;

    function buildQuery() {
        var params = new URLSearchParams();
        var formData = new FormData(form);
        formData.forEach(function (value, key) {
            if (value !== "") {
                params.append(key, value.toString());
            }
        });
        params.set("source", currentSource);
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
        if (timeoutId) {
            window.clearTimeout(timeoutId);
        }
        timeoutId = window.setTimeout(fetchArtists, 250);
    }

    inputs.forEach(function (input) {
        var type = input.tagName.toLowerCase() === "select" ? "select" : input.type;
        var eventName = (type === "text" || type === "number") ? "input" : "change";
        input.addEventListener(eventName, scheduleFetch);
    });

    form.addEventListener("submit", function (event) {
        event.preventDefault();
        if (timeoutId) {
            window.clearTimeout(timeoutId);
        }
        fetchArtists();
    });
})();
