(function () { // IIFE to avoid leaking globals
    // Shared modal helpers used by both Spotify and Deezer popups
    function showModal(modalRoot, iframe, externalLink, externalUrl, embedUrl) {
        // Keep the external link available even if the iframe can't load
        if (externalLink) externalLink.href = externalUrl || "#";
        if (iframe) iframe.src = embedUrl || "";

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
        if (iframe) iframe.src = "";
        document.body.classList.remove("overflow-hidden");
    }

    // Expose a tiny API for the source-specific modal scripts
    window.GroupieEmbedModal = {
        showModal,
        hideModal
    };
})();
