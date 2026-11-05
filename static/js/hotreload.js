"use strict";

// Reloads the page when the server reports changed UI files. Stops silently when the server
// answers 404, which means hot reload is disabled.
(function () {
    let version = null;
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
