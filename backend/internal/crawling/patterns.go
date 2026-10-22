package crawling

import "regexp"

var (
	slugPattern        = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	imdbPattern        = regexp.MustCompile(`^tt[0-9]{7,10}$`)
	contentHashPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)
)
