package editor

const (
	configWrapLeft  = true // Allow wrapping cursor to the previous line with 'h' motion.
	configWrapRight = true // Allow wrapping cursor to the next line with 'l' motion.

	// TODO keep synched with utils/utils.go/TabstopLength
	tabstopLength           = 8
	viewVerticalThreshold   = 5
	viewHorizontalThreshold = 10
)
