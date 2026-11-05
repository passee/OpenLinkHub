# Headset audio on Windows

## Current setup

The HS80 EQ and the microphone boost run through [Equalizer APO](https://sourceforge.net/projects/equalizerapo/).
OpenLinkHub writes `openlinkhub.txt` into Equalizer APO's config folder and includes it from `config.txt`.

- **EQ**: ten peaking filters on the HS80 headphones, selected by the endpoint GUID.
- **Mic boost**: a preamp of 0 to 20 dB on the HS80 microphone, also selected by GUID. Equalizer APO must be
  installed on the microphone: open its Device Selector, tick the HS80 microphone under *Capture devices*,
  click OK and restart Windows.

Equalizer APO keeps Corsair's own audio effect running as a child effect, so nothing of the Corsair driver is lost.

## Possible replacement: Corsair's audio effect

iCUE does not use Equalizer APO. The Corsair iCUE Audio Drivers install their own audio processing object
(`CorsairGamingAudioPO64.dll`, CLSIDs `{a68deea6-139a-42eb-b487-f766e45f28bb}` for output and
`{1d796f55-1ac6-4dfc-82cd-00fe8cdba4a9}` for input) plus `CorsairGamingAudioCfgService64.exe`.
Driving that effect directly would make the EQ sound exactly like iCUE and remove the Equalizer APO dependency.

What is known so far:

- iCUE talks to the effect over named pipes (`\\.\pipe\...`); running servers are listed under
  `HKLM\Software\Corsair\Audio\Servers`, device settings under `HKLM\Software\Corsair\Audio\Devices`.
- The protocol has about 40 packet types (`IPCAudioDevicePacket*`): device open/close, get/set profile,
  get/set block, volume, stream format and more.
- Audio is built as a graph of blocks (`PeakingEQFilterBlock`, `GainBlock`, `LimiterBlock`, `CompressorBlock`,
  `SoundIDBlock`, ...). The EQ is one block of a profile that iCUE sends with `SetProfile`/`SetBlock`.
- The microphone already runs Corsair's input effect, whose `GainBlock` could provide the mic boost.

Steps if this is picked up:

1. Remove Equalizer APO from the HS80 headphones so Corsair's effect is the only one again.
2. Start iCUE once and record the pipe traffic while switching EQ presets and the mic level.
3. Decode the profile and block serialization, then send the same packets from OpenLinkHub.

Estimated effort: 4 to 8 hours. Risk: a Corsair audio driver update can change the protocol.

The driver packages for a fresh Windows install can be copied from the driver store
(`corsairgamingaudio*`, `scufdriver5`, `scufdriver6`) and installed with `pnputil /add-driver *.inf /subdirs /install`.
They are Corsair's and must not be redistributed.
