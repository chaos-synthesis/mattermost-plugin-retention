package store

import (
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/mattermost/mattermost/server/public/model"
)

type StalePostOpts struct {
	AgeInDays float64
	UserId    string
}

func (ss *SQLStore) GetStalePosts(opts StalePostOpts, page int, pageSize int) ([]string, bool, error) {
	olderThan := model.GetMillisForTime(time.Now().Add(-1 * time.Duration(opts.AgeInDays*24.*float64(time.Hour))))

	// find all channels where no posts or reactions have been modified,deleted since the olderThan timestamp.
	query := ss.builder.Select("p.Id").Distinct().
		From("Posts as p").
		Where(sq.And{
			sq.Eq{"p.UserId": opts.UserId},
			sq.Lt{"p.UpdateAt": olderThan},
			sq.Eq{"p.DeleteAt": 0},
		}).
		GroupBy("p.Id").
		OrderBy("p.Id")

	if page > 0 {
		query = query.Offset(uint64(page) * uint64(pageSize)) //nolint:gosec // page and pageSize are validated to be non-negative
	}

	if pageSize > 0 {
		// N+1 to check if there's a next page for pagination
		query = query.Limit(uint64(pageSize) + 1)
	}

	rows, err := query.Query()
	if err != nil {
		ss.logger.Error("error fetching stale posts", "err", err)
		return nil, false, err
	}

	posts := []string{}
	for rows.Next() {
		post := &model.Post{}

		if err := rows.Scan(&post.Id); err != nil {
			ss.logger.Error("error scanning stale posts", "err", err)
			return nil, false, err
		}
		posts = append(posts, post.Id)
	}

	var hasMore bool
	if pageSize > 0 && len(posts) > pageSize {
		hasMore = true
		posts = posts[0:pageSize]
	}

	return posts, hasMore, nil
}
