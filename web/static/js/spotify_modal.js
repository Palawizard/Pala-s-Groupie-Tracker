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

        openSpotify.href = spotifyUrl || "#";
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
