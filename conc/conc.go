package conc

// Args encapsulates the arguments that will be passed to every task that we
// want to concurrently run. It should be noted that this is expected to be
// immutable for the duration of time that it takes to process all tasks of a
// like kind.
type Args struct {
	RootPath   string
	SiteID     int64
	OriginID   int64
	ItemTypeID int64
}
