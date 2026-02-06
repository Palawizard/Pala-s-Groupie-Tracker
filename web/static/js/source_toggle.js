(function () { // IIFE to keep this script self-contained
    // Source toggle in the header, keeps the current page when possible
    const toggle = document.getElementById("source-toggle");
    if (!toggle) return;

    function getBasePath() {
        const bp = document.body ? (document.body.getAttribute("data-base-path") || "") : "";
        return String(bp || "").replace(/\/+$/, "");
    }

    // handleClick switches the `source` query parameter and navigates
    function handleClick(event) {
        // Support clicks on nested elements inside the button
        const btn = event.target && event.target.closest ? event.target.closest("button[data-target]") : null;
        if (!btn) return;

        const source = btn.getAttribute("data-target");
        if (!source) return;

        const basePath = getBasePath();
        const fullPath = window.location.pathname || "/";
        const path = (basePath && fullPath.indexOf(basePath) === 0) ? (fullPath.slice(basePath.length) || "/") : fullPath;
        const isArtistDetail = /^\/artists\/[^/]+$/.test(path) && path !== "/artists/ajax";

        // Artist detail pages don't have stable IDs across sources, so go back to search
        if (isArtistDetail) {
            const url = new URL((basePath || "") + "/artists", window.location.origin);
            url.searchParams.set("source", source);

            const name = (document.querySelector("main h1")?.textContent || "").trim();
            if (name) {
                // Best-effort: prefill the search box with the current artist name
                url.searchParams.set("q", name);
            }

            window.location.href = url.toString();
            return;
        }

        // For list and home pages, keep the current URL and just swap the source
        const url = new URL(window.location.href);
        url.searchParams.set("source", source);
        window.location.href = url.toString();
    }

    toggle.addEventListener("click", handleClick);
})();

