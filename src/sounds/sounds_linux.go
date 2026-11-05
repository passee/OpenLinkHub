package sounds

// Play is a no-op on Linux, where iCUE's prompt files are not installed.
func Play(name, device string) {}
