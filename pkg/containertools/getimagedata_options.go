package containertools

type GetImageDataOptions struct {
	WorkingDir string
	SkipTLS    bool
}

type GetImageDataOption func(*GetImageDataOptions)

func WithWorkingDir(workingDir string) GetImageDataOption {
	return func(o *GetImageDataOptions) {
		o.WorkingDir = workingDir
	}
}

func SkipTLS(skipTLS bool) GetImageDataOption {
	return func(o *GetImageDataOptions) {
		o.SkipTLS = skipTLS
	}
}
