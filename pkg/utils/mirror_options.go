package utils

// MirrorOptions describes which artifact categories to mirror.
type MirrorOptions struct {
	IncludeImages bool
	IncludeModels bool
	IncludeCharts bool
}

// NormalizeMirrorOptions ensures at least one artifact category is included.
func NormalizeMirrorOptions(opts MirrorOptions) MirrorOptions {
	if !opts.IncludeImages && !opts.IncludeModels && !opts.IncludeCharts {
		return MirrorOptions{
			IncludeImages: true,
			IncludeModels: true,
			IncludeCharts: true,
		}
	}
	return opts
}

// MirrorOptionsFromPull converts pull options to mirror options.
func MirrorOptionsFromPull(opts PullOptions) MirrorOptions {
	return MirrorOptions{
		IncludeImages: opts.IncludeImages,
		IncludeModels: opts.IncludeModels,
		IncludeCharts: opts.IncludeCharts,
	}
}
