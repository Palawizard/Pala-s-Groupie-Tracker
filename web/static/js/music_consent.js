(function () { // IIFE to avoid leaking globals
    // Third-party music content (Spotify/Deezer players, Apple/Spotify/Deezer audio previews)
    // only loads once the visitor allows the "music" purpose of the shared palawi.fr
    // consent manager (/consent/palawi-consent.js). Without that script, nothing loads
    // until the visitor explicitly clicks to load it.
    const PURPOSE = "music";

    // fallback shows a small notice with a button; load() runs only on that click
    function fallback(target, provider, load) {
        let done = false;
        const box = document.createElement("div");
        box.setAttribute("data-music-consent-fallback", "");
        box.className = "flex flex-col items-center justify-center gap-2 rounded-xl border border-slate-300 bg-slate-100 p-4 text-center text-xs text-slate-600 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-300";

        const text = document.createElement("p");
        text.textContent = "This content is hosted by " + provider + ", which may set cookies. It only loads if you ask for it.";

        const btn = document.createElement("button");
        btn.type = "button";
        btn.className = "inline-flex items-center rounded-full border border-slate-300 px-3 py-1 text-xs font-medium text-emerald-600 hover:bg-slate-200 transition-colors dark:border-slate-700 dark:text-emerald-400 dark:hover:bg-slate-800/90";
        btn.textContent = "Load content";
        btn.addEventListener("click", () => {
            if (done) return;
            done = true;
            box.remove();
            load();
        });

        box.append(text, btn);
        target.appendChild(box);

        return function cancel() {
            done = true;
            box.remove();
        };
    }

    // gate runs load() now if music is allowed, otherwise shows a placeholder in
    // target and runs load() once allowed. Returns a cancel function.
    function gate(target, provider, load) {
        const consent = window.PalawiConsent;
        if (consent && typeof consent.gate === "function") {
            return consent.gate(target, PURPOSE, load, { provider: provider });
        }
        return fallback(target, provider, load);
    }

    // Audio previews: <audio data-consent-src> stay hidden and without a source until
    // the section's [data-preview-gate] slot is allowed
    function setupPreviews() {
        document.querySelectorAll("[data-preview-gate]").forEach((slot) => {
            const section = slot.closest("[data-preview-section]") || document;
            const audios = Array.from(section.querySelectorAll("audio[data-consent-src]"));
            if (!audios.length) {
                slot.remove();
                return;
            }

            slot.classList.remove("hidden");
            gate(slot, slot.getAttribute("data-provider") || "a third party", () => {
                slot.remove();
                audios.forEach((audio) => {
                    audio.src = audio.getAttribute("data-consent-src") || "";
                    audio.removeAttribute("data-consent-src");
                    audio.classList.remove("hidden");
                });
            });
        });
    }

    window.GroupieMusicConsent = { gate };

    if (document.readyState === "loading") {
        document.addEventListener("DOMContentLoaded", setupPreviews);
    } else {
        setupPreviews();
    }
})();
