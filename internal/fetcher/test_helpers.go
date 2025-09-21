package fetcher

import "github.com/rs/zerolog"

func noopLogger() zerolog.Logger {
	return zerolog.Nop()
}
