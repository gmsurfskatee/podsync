package builder

import (
	"net/http"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/silentsokolov/go-vimeo/vimeo"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"

	"github.com/mxpv/podsync/pkg/api"
	"github.com/mxpv/podsync/pkg/config"
	"github.com/mxpv/podsync/pkg/link"
)

const (
	vimeoDefaultPageSize = 50
)

type VimeoBuilder struct {
	client *vimeo.Client
}

func (v *VimeoBuilder) selectImage(p *vimeo.Pictures, q config.Quality) string {
	if p == nil || len(p.Sizes) == 0 {
		return ""
	}

	if q == config.QualityLow {
		return p.Sizes[0].Link
	}

	return p.Sizes[len(p.Sizes)-1].Link
}

func (v *VimeoBuilder) queryChannel(feed *Feed, cfg *config.Feed) error {
	channelID := feed.ItemID

	ch, resp, err := v.client.Channels.Get(channelID)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return api.ErrNotFound
		}

		return errors.Wrapf(err, "failed to query channel with id %q", channelID)
	}

	feed.Title = ch.Name
	feed.ItemURL = ch.Link
	feed.Description = ch.Description
	feed.CoverArt = v.selectImage(ch.Pictures, cfg.Quality)
	feed.Author = ch.User.Name
	feed.PubDate = ch.CreatedTime
	feed.UpdatedAt = time.Now().UTC()

	return nil
}

func (v *VimeoBuilder) queryGroup(feed *Feed, cfg *config.Feed) error {
	groupID := feed.ItemID

	gr, resp, err := v.client.Groups.Get(groupID)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return api.ErrNotFound
		}

		return errors.Wrapf(err, "failed to query group with id %q", groupID)
	}

	feed.Title = gr.Name
	feed.ItemURL = gr.Link
	feed.Description = gr.Description
	feed.CoverArt = v.selectImage(gr.Pictures, cfg.Quality)
	feed.Author = gr.User.Name
	feed.PubDate = gr.CreatedTime
	feed.UpdatedAt = time.Now().UTC()

	return nil
}

func (v *VimeoBuilder) queryUser(feed *Feed, cfg *config.Feed) error {
	userID := feed.ItemID

	user, resp, err := v.client.Users.Get(userID)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return api.ErrNotFound
		}

		return errors.Wrapf(err, "failed to query user with id %q", userID)
	}

	feed.Title = user.Name
	feed.ItemURL = user.Link
	feed.Description = user.Bio
	feed.CoverArt = v.selectImage(user.Pictures, cfg.Quality)
	feed.Author = user.Name
	feed.PubDate = user.CreatedTime
	feed.UpdatedAt = time.Now().UTC()

	return nil
}

func (v *VimeoBuilder) getVideoSize(video *vimeo.Video) int64 {
	// Very approximate video file size
	return int64(float64(video.Duration*video.Width*video.Height) * 0.38848958333)
}

type getVideosFunc func(string, ...vimeo.CallOption) ([]*vimeo.Video, *vimeo.Response, error)

func (v *VimeoBuilder) queryVideos(getVideos getVideosFunc, feed *Feed, cfg *config.Feed) error {
	var (
		page  = 1
		added = 0
	)

	for {
		videos, response, err := getVideos(feed.ItemID, vimeo.OptPage(page), vimeo.OptPerPage(vimeoDefaultPageSize))
		if err != nil {
			if response != nil {
				return errors.Wrapf(err, "failed to query videos (error %d %s)", response.StatusCode, response.Status)
			}

			return err
		}

		for _, video := range videos {
			var (
				videoID  = strconv.Itoa(video.GetID())
				videoURL = video.Link
				duration = int64(video.Duration)
				size     = v.getVideoSize(video)
				image    = v.selectImage(video.Pictures, cfg.Quality)
			)

			feed.Episodes = append(feed.Episodes, &Item{
				ID:          videoID,
				Title:       video.Name,
				Description: video.Description,
				Duration:    duration,
				Size:        size,
				PubDate:     video.CreatedTime,
				Thumbnail:   image,
				VideoURL:    videoURL,
			})

			added++
		}

		if added >= cfg.PageSize || response.NextPage == "" {
			return nil
		}

		page++
	}
}

func (v *VimeoBuilder) Build(cfg *config.Feed) (*Feed, error) {
	info, err := link.Parse(cfg.URL)
	if err != nil {
		return nil, err
	}

	feed := &Feed{}

	if info.LinkType == link.TypeChannel {
		if err := v.queryChannel(feed, cfg); err != nil {
			return nil, err
		}

		if err := v.queryVideos(v.client.Channels.ListVideo, feed, cfg); err != nil {
			return nil, err
		}

		return feed, nil
	}

	if info.LinkType == link.TypeGroup {
		if err := v.queryGroup(feed, cfg); err != nil {
			return nil, err
		}

		if err := v.queryVideos(v.client.Groups.ListVideo, feed, cfg); err != nil {
			return nil, err
		}

		return feed, nil
	}

	if info.LinkType == link.TypeUser {
		if err := v.queryUser(feed, cfg); err != nil {
			return nil, err
		}

		if err := v.queryVideos(v.client.Users.ListVideo, feed, cfg); err != nil {
			return nil, err
		}

		return feed, nil
	}

	return nil, errors.New("unsupported feed type")
}

func NewVimeoBuilder(ctx context.Context, token string) (*VimeoBuilder, error) {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)

	client := vimeo.NewClient(tc, nil)
	return &VimeoBuilder{client}, nil
}