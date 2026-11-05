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

    function hexToColor(hex) {
        return {
            red: parseInt(hex.slice(1, 3), 16),
            green: parseInt(hex.slice(3, 5), 16),
            blue: parseInt(hex.slice(5, 7), 16),
            brightness: 1
        };
    }

    function saveMicLed() {
        $("#micLedActiveColorBox").toggle($("#micLedActive").val() === "color");
        $.ajax({
            url: "/api/headset/micLed",
            type: "POST",
            contentType: "application/json",
            data: JSON.stringify({
                deviceId: $("#deviceId").val(),
                active: $("#micLedActive").val(),
                activeColor: hexToColor($("#micLedActiveColor").val()),
                mutedColor: hexToColor($("#micLedMutedColor").val())
            }),
            success(response) {
                if (response?.status === 1) {
                    toast.success(response.message);
                } else {
                    toast.warning(response?.message || "Operation failed");
                }
            },
            error() {
                toast.warning("Request failed");
            }
        });
    }
    $("#micLedActive, #micLedActiveColor, #micLedMutedColor").on("change", saveMicLed);

    $("#micBoostSlider").on("input", function () {
        $("#micBoostSliderValue").text("+" + $(this).val() + " dB");
    }).on("change", function () {
        $.ajax({
            url: "/api/headset/micBoost",
            type: "POST",
            contentType: "application/json",
            data: JSON.stringify({ boost: parseFloat($(this).val()) }),
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
        const presets = {};
        $.getJSON("/api/headset/eqPresets", function (response) {
            (response?.data || []).forEach(function (preset) {
                presets[preset.name] = preset.bands || [];
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
                        (presets[name] || []).forEach(function (gain, i) {
                            $("#stage" + (i + 1)).val(gain);
                            $("#stageValue" + (i + 1)).text(gain);
                        });
                        toast.success(response.message || "Preset applied");
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

    $(".toggleEqClippingProtection").on("change", function () {
        const $toggle = $(this);
        postToggle($toggle, "/api/headset/eqClippingProtection", { enabled: $toggle.prop("checked") });
    });

    $("#keyboardBrightnessSlider").on("input", function () {
        $("#keyboardBrightnessSliderValue").text($(this).val() + " %");
    }).on("change", function () {
        $.ajax({
            url: "/api/keyboard/dial/options",
            type: "POST",
            contentType: "application/json",
            data: JSON.stringify({ deviceId: $("#deviceId").val(), brightness: parseInt($(this).val(), 10) }),
            success(response) {
                if (response?.status !== 1) {
                    toast.warning(response?.message || "Operation failed");
                }
            }
        });
    });

    $(".toggleDialSyncColor").on("change", function () {
        const $toggle = $(this);
        postToggle($toggle, "/api/keyboard/dial/options", {
            deviceId: $("#deviceId").val(),
            syncColor: $toggle.prop("checked")
        });
    });

    // Saved together with the other zone colours; the zone save shows the confirmation.
    $("#saveZoneColors").on("click", function () {
        const $picker = $("#sniperDpiColor");
        if (!$picker.length) {
            return;
        }
        const hex = $picker.val();
        $.ajax({
            url: "/api/mouse/sniperColor",
            type: "POST",
            contentType: "application/json",
            data: JSON.stringify({
                deviceId: $("#deviceId").val(),
                red: parseInt(hex.substr(1, 2), 16),
                green: parseInt(hex.substr(3, 2), 16),
                blue: parseInt(hex.substr(5, 2), 16)
            }),
            success(response) {
                if (response?.status !== 1) {
                    toast.warning(response?.message || "Unable to save sniper colour");
                }
            }
        });
    });

    $(".toggleGlobalDpi").on("change", function () {
        const $toggle = $(this);
        postToggle($toggle, "/api/mouse/globalDpi", {
            deviceId: $("#deviceId").val(),
            enabled: $toggle.prop("checked")
        });
    });
});
