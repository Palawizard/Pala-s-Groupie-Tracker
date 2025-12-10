(function () {
    const form = document.getElementById("artist-filters");
    const list = document.getElementById("artist-list");
    if (!form || !list) return;

    const inputs = form.querySelectorAll("input");
    let timeoutId;

    function buildQuery() {
        const params = new URLSearchParams();
        const formData = new FormData(form);
        for (const [key, value] of formData.entries()) {
            if (value !== "") {
                params.append(key, value.toString());
            }
        }
        params.append("format", "json");
        return params.toString();
    }

    function renderArtists(artists) {
        if (!Array.isArray(artists) || artists.length === 0) {
            list.innerHTML = '<p class="text-sm text-slate-400 col-span-full">No artists match these filters.</p>';
            return;
        }

        const html = artists.map(function (a) {
            const membersCount = Array.isArray(a.members) ? a.members.length : 0;
            return (
                '<a href="/artists/' + a.id + '" class="block rounded-lg border border-slate-800 bg-slate-900/60 p-3 hover:border-emerald-500/70 hover:bg-slate-900 transition-colors">' +
                '<article class="flex flex-col gap-2">' +
                '<img src="' + a.image + '" alt="' + a.name + '" class="w-full h-40 object-cover rounded-md">' +
                '<h2 class="text-base font-semibold">' + a.name + "</h2>" +
                '<p class="text-xs text-slate-400">Creation date: ' + a.creationDate + "</p>" +
                '<p class="text-xs text-slate-400">Members: ' + membersCount + "</p>" +
                "</article>" +
                "</a>"
            );
        }).join("");

        list.innerHTML = html;
    }

    function fetchArtists() {
        const query = buildQuery();
        const url = "/artists?" + query;

        fetch(url, {
            headers: {
                "Accept": "application/json"
            }
        })
            .then(function (res) {
                if (!res.ok) {
                    throw new Error("request failed");
                }
                return res.json();
            })
            .then(function (data) {
                renderArtists(data);
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
        const eventName = input.type === "text" || input.type === "number" ? "input" : "change";
        input.addEventListener(eventName, scheduleFetch);
    });
})();
