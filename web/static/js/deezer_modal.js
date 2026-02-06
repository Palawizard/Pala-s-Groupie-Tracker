(function () { // IIFE to avoid leaking globals
    // Deezer modal wiring, relies on GroupieEmbedModal from embed_modal.js
    const modalRoot = document.getElementById("deezer_modal_root");
    const backdrop = document.getElementById("deezer_modal_backdrop");
    const iframe = document.getElementById("deezer_modal_iframe");
    const closeBtn = document.getElementById("deezer_modal_close_btn");
    const openDeezer = document.getElementById("deezer_modal_open_deezer");

    if (!modalRoot || !backdrop || !iframe || !closeBtn || !openDeezer) {
        return;
    }

    // toEmbedUrl builds a Deezer widget URL for a given type and ID
    function toEmbedUrl(type, id) {
        const safeType = (type === "track" || type === "album" || type === "playlist" || type === "artist") ? type : "track";
        const safeID = String(id || "").trim();
        if (!safeID) return "";
        return "https://widget.deezer.com/widget/dark/" + safeType + "/" + encodeURIComponent(safeID);
    }

    // openModal sets the iframe src and shows the modal
    function openModal(payload) {
        const deezerUrl = payload.deezerUrl || "";
        const type = payload.type || "track";
        const id = payload.id || "";
        const embedUrl = toEmbedUrl(type, id);

        if (!embedUrl) return;
        if (!window.GroupieEmbedModal) return;

        window.GroupieEmbedModal.showModal(modalRoot, iframe, openDeezer, deezerUrl, embedUrl);
    }

    // closeModal hides the modal and clears the iframe
    function closeModal() {
        if (!window.GroupieEmbedModal) return;
        window.GroupieEmbedModal.hideModal(modalRoot, iframe);
    }

    // onKeyDown closes the modal on Escape
    function onKeyDown(e) {
        if (e.key === "Escape") closeModal();
    }

    // onDocumentClick uses event delegation for Deezer buttons
    function onDocumentClick(e) {
        const btn = e.target && e.target.closest ? e.target.closest("[data-deezer-open]") : null;
        if (!btn) return;

        e.preventDefault();

        openModal({
            deezerUrl: btn.getAttribute("data-deezer-url") || "",
            type: btn.getAttribute("data-deezer-type") || "track",
            id: btn.getAttribute("data-deezer-id") || ""
        });
    }

    backdrop.addEventListener("click", closeModal);
    closeBtn.addEventListener("click", closeModal);
    document.addEventListener("keydown", onKeyDown);
    document.addEventListener("click", onDocumentClick);
})();
