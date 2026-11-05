"use strict";

// Reloads the page when the server reports changed UI files and shows a small badge while hot
// reload is active. Stops silently when the server answers 404, which means it is disabled.
(function () {
    let version = null;

    function showBadge() {
        const badge = document.createElement("div");
        badge.textContent = "Hot reload active · " + new Date().toLocaleTimeString();
        badge.style.cssText = "position:fixed;right:8px;bottom:8px;z-index:99999;padding:2px 8px;" +
            "border-radius:6px;font:12px sans-serif;background:rgba(40,167,69,.85);color:#fff;" +
            "pointer-events:none";
        document.body.appendChild(badge);
    }

    async function poll() {
        try {
            const url = "/api/dev/reload" + (version === null ? "" : "?since=" + version);
            const response = await fetch(url, { cache: "no-store" });
            if (response.status === 404) {
                return;
            }
            const data = await response.json();
            if (version !== null && data.version !== version) {
                location.reload();
                return;
            }
            if (version === null) {
                showBadge();
            }
            version = data.version;
            poll();
        } catch (e) {
            // Server restarting: retry, and reload once it answers again.
            version = version === null ? null : -1;
            setTimeout(poll, 1000);
        }
    }
    poll();
})();
