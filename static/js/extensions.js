"use strict";

$(document).ready(function () {
    function postToggle($toggle, url, payload) {
        const previousState = !$toggle.prop("checked");
        $toggle.prop("disabled", true);
        $.ajax({
            url: url,
            type: "POST",
            contentType: "application/json",
            data: JSON.stringify(payload),
            success(response) {
                if (response?.status !== 1) {
                    $toggle.prop("checked", previousState);
                    toast.warning(response?.message || "Operation failed");
                } else {
                    toast.success(response?.message || "Saved");
                }
            },
            error() {
                $toggle.prop("checked", previousState);
                toast.warning("Request failed");
            },
            complete() {
                $toggle.prop("disabled", false);
            }
        });
    }

    $(".toggleGlobalDpi").on("change", function () {
        const $toggle = $(this);
        postToggle($toggle, "/api/mouse/globalDpi", {
            deviceId: $("#deviceId").val(),
            enabled: $toggle.prop("checked")
        });
    });
});
