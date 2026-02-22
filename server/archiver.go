package main

import (
	"context"
	"fmt"
	"time"

	"github.com/chaos-synthesis/mattermost-plugin-retention/server/mmctl/commands"
	"github.com/chaos-synthesis/mattermost-plugin-retention/server/store"
)

type Reason string

const (
	ReasonDone      Reason = "completed normally"
	ReasonCancelled Reason = "canceled"
	ReasonError     Reason = "error"
)

type ArchiverOpts struct {
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

func (p *Plugin) RemoveUserStalePosts(ctx context.Context, opts ArchiverOpts) (results *ArchiverResults, retErr error) {
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

	maxWarns := opts.MaxWarnings
	if maxWarns <= 0 {
		maxWarns = 100
	}

	userIds := p.kvStore.GetActiveUsers()
	p.API.LogDebug("Removing stale posts.", "usersCount", len(userIds))

	for _, userId := range userIds {
		userPrefs, err := p.kvStore.GetUserSettings(userId)
		if err != nil {
			p.API.LogError("Cannot fetch user settings for user", "userId", userId, "error", err)
			continue
		} else if !userPrefs.Enabled {
			p.API.LogDebug("Skipping user with post deletion disabled", "userId", userId)
			continue
		}

		failsCount := 0
		for {
			postOpts := store.StalePostOpts{
				AgeInDays: userPrefs.PostAgeInDays,
				UserId:    userId,
			}
			posts, more, err := p.sqlStore.GetStalePosts(postOpts, 0, opts.BatchSize)

			if err != nil {
				results.ExitReason = ReasonError
				p.API.LogError("Cannot fetch stale posts", "error", err)
				return results, fmt.Errorf("cannot fetch stale posts: %w", err)
			}

			if len(posts) > 0 {
				cmdLine := append([]string{"post", "delete"}, posts...)
				if err := commands.Run(append(cmdLine, "--permanent", "--confirm", "--local", "--quiet")); err != nil {
					p.API.LogError("Cannot remove stale posts", "error", err)

					failsCount++

					if failsCount > maxWarns {
						results.ExitReason = ReasonError
						p.API.LogError("Cannot remove stale posts", "error", err)

						return results, fmt.Errorf("cannot remove stale posts: %w", err)
					}
				}

				results.PostsDeleted += len(posts)
			}

			p.API.LogInfo("Removed stale posts", "posts", results.PostsDeleted)

			if !more {
				break
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

	return results, nil
}
