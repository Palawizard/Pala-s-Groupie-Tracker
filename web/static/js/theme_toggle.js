(function () { // IIFE to avoid leaking globals
    const STORAGE_KEY = "theme"; // "light" | "dark" | "system"
    const root = document.documentElement;
    const btn = document.getElementById("theme-toggle");
    const mql = window.matchMedia ? window.matchMedia("(prefers-color-scheme: dark)") : null;

    function safeGetTheme() {
        try {
            return localStorage.getItem(STORAGE_KEY);
        } catch (e) {
            return null;
        }
    }

    function safeSetTheme(v) {
        try {
            localStorage.setItem(STORAGE_KEY, v);
        } catch (e) {
            // Ignore storage errors (private mode, disabled storage, etc.)
        }
    }

    function resolveTheme(pref) {
        if (pref === "dark" || pref === "light") return pref;
        const prefersDark = !!(mql && mql.matches);
        return prefersDark ? "dark" : "light";
    }

    function applyTheme(pref) {
        const t = resolveTheme(pref);
        root.classList.toggle("dark", t === "dark");
        if (btn) {
            btn.setAttribute("aria-label", t === "dark" ? "Switch to light mode" : "Switch to dark mode");
            btn.setAttribute("data-theme", t);
        }
    }

    // Default to dark to preserve the repo's original dark-first UI.
    applyTheme(safeGetTheme() || "dark");

    if (btn) {
        btn.addEventListener("click", function () {
            const current = root.classList.contains("dark") ? "dark" : "light";
            const next = current === "dark" ? "light" : "dark";
            safeSetTheme(next);
            applyTheme(next);
        });
    }

    if (mql && typeof mql.addEventListener === "function") {
        mql.addEventListener("change", function () {
            const pref = safeGetTheme();
            if (!pref || pref === "system") applyTheme("system");
        });
    }
})();
