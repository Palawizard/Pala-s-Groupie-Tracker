(function () {
    const modalRoot = document.getElementById("spotify_modal_root");
    const backdrop = document.getElementById("spotify_modal_backdrop");
    const iframe = document.getElementById("spotify_modal_iframe");
    const closeBtn = document.getElementById("spotify_modal_close_btn");
    const openSpotify = document.getElementById("spotify_modal_open_spotify");

    if (!modalRoot || !backdrop || !iframe || !closeBtn || !openSpotify) {
        return;
    }

    function toEmbedUrl(spotifyUrl) {
        try {
            const u = new URL(spotifyUrl);
            const parts = u.pathname.split("/").filter(Boolean);
            if (parts.length >= 2) {
                return `${u.origin}/embed/${parts[0]}/${parts[1]}`;
            }
            return `${u.origin}/embed`;
        } catch (e) {
            return spotifyUrl;
        }
    }

    function openModal(payload) {
        const spotifyUrl = payload.spotifyUrl || "";
        const embedUrl = toEmbedUrl(spotifyUrl);
        if (!embedUrl) return;
        if (!window.GroupieEmbedModal) return;

        window.GroupieEmbedModal.showModal(modalRoot, iframe, openSpotify, spotifyUrl, embedUrl);
    }

    function closeModal() {
        if (!window.GroupieEmbedModal) return;
        window.GroupieEmbedModal.hideModal(modalRoot, iframe);
    }

    function onKeyDown(e) {
        if (e.key === "Escape") closeModal();
    }

    document.querySelectorAll("[data-spotify-open]").forEach((btn) => {
        btn.addEventListener("click", (e) => {
            e.preventDefault();
            openModal({
                spotifyUrl: btn.getAttribute("data-spotify-url") || ""
            });
        });
    });

    backdrop.addEventListener("click", closeModal);
    closeBtn.addEventListener("click", closeModal);
    document.addEventListener("keydown", onKeyDown);
})();
