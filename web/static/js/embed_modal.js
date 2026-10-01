(function () { // IIFE to avoid leaking globals
    // Shared modal helpers used by both Spotify and Deezer popups
    // Pending consent placeholders, keyed by modal root, cancelled when the modal closes
    const pendingGates = new WeakMap();

    function cancelGate(modalRoot) {
        const cancel = pendingGates.get(modalRoot);
        if (cancel) cancel();
        pendingGates.delete(modalRoot);
    }

    // showModal opens the modal; the player iframe only gets its src once the
    // visitor allows music players (see music_consent.js). The iframe's parent
    // element is the slot where the consent placeholder goes.
    function showModal(modalRoot, iframe, externalLink, externalUrl, embedUrl, provider) {
        // Keep the external link available even if the iframe can't load
        if (externalLink) externalLink.href = externalUrl || "#";

        cancelGate(modalRoot);
        if (iframe) {
            iframe.removeAttribute("src");
            iframe.classList.add("hidden");

            const load = () => {
                iframe.classList.remove("hidden");
                iframe.src = embedUrl || "";
            };
            // Never load the third-party player without the consent helper
            if (embedUrl && window.GroupieMusicConsent) {
                pendingGates.set(modalRoot, window.GroupieMusicConsent.gate(iframe.parentElement, provider, load));
            }
        }

        // Toggle visibility with utility classes to keep CSS simple
        modalRoot.classList.remove("hidden");
        modalRoot.classList.add("flex");
        modalRoot.setAttribute("aria-hidden", "false");
        document.body.classList.add("overflow-hidden");
    }

    // hideModal closes the modal and clears the iframe to stop playback
    function hideModal(modalRoot, iframe) {
        modalRoot.classList.add("hidden");
        modalRoot.classList.remove("flex");
        modalRoot.setAttribute("aria-hidden", "true");
        cancelGate(modalRoot);
        if (iframe) {
            iframe.src = "about:blank";
            iframe.classList.add("hidden");
        }
        document.body.classList.remove("overflow-hidden");
    }

    // Expose a tiny API for the source-specific modal scripts
    window.GroupieEmbedModal = {
        showModal,
        hideModal
    };
})();
