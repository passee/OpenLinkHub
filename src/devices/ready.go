package devices

var initialized = make(chan struct{})

// Initialized is closed once every device found at startup finished its initialization.
func Initialized() <-chan struct{} {
	return initialized
}

func closeWhenInitialized() {
	go func() {
		initWG.Wait()
		close(initialized)
	}()
}
