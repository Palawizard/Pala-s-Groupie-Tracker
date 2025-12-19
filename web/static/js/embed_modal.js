(function () {
    function showModal(modalRoot, iframe, externalLink, externalUrl, embedUrl) {
        if (externalLink) externalLink.href = externalUrl || "#";
        if (iframe) iframe.src = embedUrl || "";

        modalRoot.classList.remove("hidden");
        modalRoot.classList.add("flex");
        modalRoot.setAttribute("aria-hidden", "false");
        document.body.classList.add("overflow-hidden");
    }

    function hideModal(modalRoot, iframe) {
        modalRoot.classList.add("hidden");
        modalRoot.classList.remove("flex");
        modalRoot.setAttribute("aria-hidden", "true");
        if (iframe) iframe.src = "";
        document.body.classList.remove("overflow-hidden");
    }

    window.GroupieEmbedModal = {
        showModal,
        hideModal
    };
})();
