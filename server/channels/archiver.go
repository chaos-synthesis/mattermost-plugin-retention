package channels

import (
	"context"
	"fmt"
	"time"

	"github.com/chaos-synthesis/mattermost-plugin-retention/server/mmctl/commands"
	"github.com/mattermost/mattermost/server/public/plugin"
	pluginapi "github.com/mattermost/mattermost/server/public/pluginapi"

	"github.com/chaos-synthesis/mattermost-plugin-retention/server/store"
)

type Reason string

const (
	ReasonDone      Reason = "completed normally"
	ReasonCancelled Reason = "canceled"
	ReasonError     Reason = "error"
)

type ArchiverOpts struct {
	StalePostOpts store.StalePostOpts

	BatchSize   int
	MaxWarnings int

	ProgressFn func(results *ArchiverResults) // optional callback to receive results per batch
}

type ArchiverResults struct {
	PostsDeleted int
	ExitReason   Reason
	Duration     time.Duration
	start        time.Time
}

func RemoveStalePostsWithApi(ctx context.Context, sqlStore *store.SQLStore, client *pluginapi.Client, opts ArchiverOpts, papi plugin.API) (results *ArchiverResults, retErr error) {
	results = &ArchiverResults{
		PostsDeleted: 0,
		ExitReason:   ReasonDone,
		start:        time.Now(),
	}

	defer func() {
		if p := recover(); p != nil {
			retErr = fmt.Errorf("panic recovered: %v", p)
		}
		if retErr != nil {
			results.ExitReason = ReasonError
		}
		results.Duration = time.Since(results.start)
	}()

	client.Log.Debug(
		"Removing stale posts.",
		"AgeInSeconds", opts.StalePostOpts.AgeInSeconds,
	)

	failsCount := 0

	for {
		postOpts := store.StalePostOpts{
			AgeInSeconds: opts.StalePostOpts.AgeInSeconds,
		}
		posts, more, err := sqlStore.GetStalePosts(postOpts, 0, opts.BatchSize)

		if err != nil {
			results.ExitReason = ReasonError
			if papi != nil {
				papi.LogError("Cannot fetch stale posts", "error", err)
			}
			return results, fmt.Errorf("cannot fetch stale posts: %w", err)
		}

		if len(posts) > 0 {
			cmdLine := append([]string{"post", "delete"}, posts...)
			if err := commands.Run(append(cmdLine, "--permanent", "--confirm", "--local", "--quiet")); err != nil {
				if papi != nil {
					papi.LogError("Cannot remove stale posts", "error", err)
				}
				failsCount++

				if failsCount > 1000 {
					results.ExitReason = ReasonError
					if papi != nil {
						papi.LogError("Cannot remove stale posts", "error", err)
					}
					return results, fmt.Errorf("cannot remove stale posts: %w", err)
				}
			}

			results.PostsDeleted += len(posts)
		}

		if !more {
			return results, nil
		}

		if papi != nil {
			papi.LogInfo("Removing stale posts", "posts", results.PostsDeleted)
		}

		// sleep so we don't peg the cpu; longer here to allow websocket events to flush
		select {
		case <-time.After(time.Second * 5):
		case <-ctx.Done():
			results.ExitReason = ReasonCancelled
			return results, nil
		}
	}
}
