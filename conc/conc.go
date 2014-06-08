package conc

import (
	"fmt"
	"sync"

	"github.com/cheggaaa/pb"
)

// Args encapsulates the arguments that will be passed to every task that we
// want to concurrently run. It should be noted that this is expected to be
// immutable for the duration of time that it takes to process all tasks of a
// like kind.
type Args struct {
	RootPath           string
	SiteID             int64
	OriginID           int64
	ItemTypeID         int64
	SiteOwnerProfileID int64
	DeletedProfileID   int64
}

// RunTasks will take a range of []int64, some function args, a function and
// the number of gophers to use, and will then process all tasks evenly across
// the number of gophers.
func RunTasks(
	ids []int64,
	args Args,
	task func(Args, int64) error,
	gophers int,
) []error {

	// Progress bar
	bar := pb.StartNew(len(ids))

	// Cancel control
	done := make(chan struct{})
	quit := false

	// IDs to process, sent via channel
	tasks := make(chan int64, len(ids)+1)

	errs := []error{}
	var wg sync.WaitGroup

	// No need to have more gophers than we have tasks
	if gophers > len(ids) {
		gophers = len(ids)
	}

	// Only fire up a set number of worker processes
	for i := 0; i < gophers; i++ {
		wg.Add(1)

		go func() {
			for id := range tasks {
				err := doTask(args, id, task, done)
				if err != nil {
					// Quit as we encountered an error
					if !quit {
						close(done)
						quit = true
					}
					errs = append(
						errs,
						fmt.Errorf("Failed on ID %d : %+v", id, err),
					)
					break
				}
				bar.Increment()
			}
			wg.Done()
		}()
	}

	for _, id := range ids {
		tasks <- id
	}
	close(tasks)

	wg.Wait()
	if !quit {
		close(done)
	}

	bar.Finish()

	return errs
}

func doTask(args Args, id int64, task func(Args, int64) error, done <-chan struct{}) error {
	select {
	case <-done:
		return fmt.Errorf("task cancelled")
	default:
		if id == 0 {
			return fmt.Errorf("id zero")
		}
		return task(args, id)
	}
}
