package matcher

import "sync"

// TemplateVarMarkerName is the template variable used to represent the marker file name in file extension patterns.
const TemplateVarMarkerName = "markerName"

var registerOnce sync.Once

func init() {
	registerOnce.Do(func() {
		RegisterFileMatcher(NewBasicFileMatcher())
		RegisterFileMatcher(NewGlobFileMatcher())
		RegisterFileMatcher(NewSidecarFileMatcher())
	})
}
