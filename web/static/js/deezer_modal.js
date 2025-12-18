(function () {
    const modalRoot = document.getElementById("deezer_modal_root");
    const backdrop = document.getElementById("deezer_modal_backdrop");
    const iframe = document.getElementById("deezer_modal_iframe");
    const closeBtn = document.getElementById("deezer_modal_close_btn");
    const openDeezer = document.getElementById("deezer_modal_open_deezer");

    if (!modalRoot || !backdrop || !iframe || !closeBtn || !openDeezer) {
        return;
    }

    function toEmbedUrl(type, id) {
        const safeType = (type === "track" || type === "album" || type === "playlist" || type === "artist") ? type : "track";
        const safeID = String(id || "").trim();
        if (!safeID) return "";
        return "https://widget.deezer.com/widget/dark/" + safeType + "/" + encodeURIComponent(safeID);
    }

    function openModal(payload) {
        const deezerUrl = payload.deezerUrl || "";
        const type = payload.type || "track";
        const id = payload.id || "";
        const embedUrl = toEmbedUrl(type, id);

        if (!embedUrl) return;

        openDeezer.href = deezerUrl || "#";
        iframe.src = embedUrl;

        modalRoot.classList.remove("hidden");
        modalRoot.classList.add("flex");
        modalRoot.setAttribute("aria-hidden", "false");
        document.body.classList.add("overflow-hidden");
    }

    function closeModal() {
        modalRoot.classList.add("hidden");
        modalRoot.classList.remove("flex");
        modalRoot.setAttribute("aria-hidden", "true");
        iframe.src = "";
        document.body.classList.remove("overflow-hidden");
    }

    function onKeyDown(e) {
        if (e.key === "Escape") closeModal();
    }

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
