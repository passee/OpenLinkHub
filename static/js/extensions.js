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

    $("#micVolumeSlider").on("input", function () {
        $("#micVolumeSliderValue").text($(this).val() + " %");
    }).on("change", function () {
        $.ajax({
            url: "/api/headset/micVolume",
            type: "POST",
            contentType: "application/json",
            data: JSON.stringify({ deviceId: $("#deviceId").val(), volume: parseInt($(this).val(), 10) }),
            success(response) {
                if (response?.status !== 1) {
                    toast.warning(response?.message || "Operation failed");
                }
            },
            error() {
                toast.warning("Request failed");
            }
        });
    });

    const $eqPresets = $("#eqPresetSelect");
    if ($eqPresets.length) {
        $.getJSON("/api/headset/eqPresets", function (response) {
            (response?.data || []).forEach(function (preset) {
                $eqPresets.append($("<option>").val(preset.name).text(preset.name));
            });
        });
        $eqPresets.on("change", function () {
            const name = $(this).val();
            if (!name) {
                return;
            }
            $.ajax({
                url: "/api/headset/eqPreset",
                type: "POST",
                contentType: "application/json",
                data: JSON.stringify({ deviceId: $("#deviceId").val(), name: name }),
                success(response) {
                    if (response?.status === 1) {
                        location.reload();
                    } else {
                        toast.warning(response?.message || "Operation failed");
                    }
                },
                error() {
                    toast.warning("Request failed");
                }
            });
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
