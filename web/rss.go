package web

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/util"
	"github.com/google/uuid"
	"github.com/gorilla/feeds"
)

// buildURL creates proper URLs based on whether SSL domain is configured
func buildURL(conf *util.AppConfig, path string) string {
	if conf.Conf.WithAp && conf.Conf.SslDomain != "" {
		return fmt.Sprintf("https://%s%s", conf.Conf.SslDomain, path)
	}
	return fmt.Sprintf("http://%s:%d%s", conf.Conf.Host, conf.Conf.HttpPort, path)
}

func GetRSS(conf *util.AppConfig, username string) (string, error) {

	var err error
	var notes *[]domain.Note
	var title string
	var createdBy string
	var email string

	link := buildURL(conf, "/feed")

	if username != "" {
		err, notes = db.GetDB().ReadNotesByUsername(username)
		if err != nil {
			log.Println(fmt.Sprintf("Could not get notes from %s!", username), err)
			return "", errors.New("error retrieving notes by username")
		}
		title = fmt.Sprintf("Stegodon Notes - %s", username)
		createdBy = username
		email = fmt.Sprintf("%s@stegodon", username)
		link = fmt.Sprintf("%s?username=%s", link, username)
		// If notes exist, use the actual createdBy from first note
		if notes != nil && len(*notes) > 0 {
			createdBy = (*notes)[0].CreatedBy
			email = fmt.Sprintf("%s@stegodon", (*notes)[0].CreatedBy)
		}
	} else {
		err, notes = db.GetDB().ReadAllNotes()
		if err != nil {
			log.Println("Could not get notes!", err)
			return "", errors.New("error retrieving notes")
		}
		title = "All Stegodon Notes"
		createdBy = "everyone"
		email = fmt.Sprintf("%s@stegodon", createdBy)
	}

	feed := &feeds.Feed{
		Title:       title,
		Link:        &feeds.Link{Href: link},
		Description: "rss feed for testing stegodon",
		Author:      &feeds.Author{Name: createdBy, Email: email},
		Created:     time.Now(),
	}

	var feedItems []*feeds.Item
	if notes != nil {
		for _, note := range *notes {
			// Skip replies - only include top-level posts
			if note.InReplyToURI != "" {
				continue
			}
			email := fmt.Sprintf("%s@stegodon", note.CreatedBy)
			// Convert Markdown links to HTML for RSS feed
			contentHTML := util.MarkdownLinksToHTML(note.Message)
			feedItems = append(feedItems,
				&feeds.Item{
					Id:      note.Id.String(),
					Title:   note.CreatedAt.Format(util.DateTimeFormat()),
					Link:    &feeds.Link{Href: buildURL(conf, fmt.Sprintf("/feed/%s", note.Id))},
					Content: contentHTML,
					Author:  &feeds.Author{Name: note.CreatedBy, Email: email},
					Created: note.CreatedAt,
				})
		}
	}

	feed.Items = feedItems
	return feed.ToRss()
}

func GetRSSItem(conf *util.AppConfig, id uuid.UUID) (string, error) {
	err, note := db.GetDB().ReadNoteId(id)

	if err != nil || note == nil {
		log.Println("Could not get note!", err)
		return "", errors.New("error retrieving note by id")
	}

	email := fmt.Sprintf("%s@stegodon", note.CreatedBy)
	url := buildURL(conf, fmt.Sprintf("/feed/%s", note.Id))

	feed := &feeds.Feed{
		Title:       "Single Stegodon Note",
		Link:        &feeds.Link{Href: url},
		Description: "rss feed for testing stegodon",
		Author:      &feeds.Author{Name: note.CreatedBy, Email: email},
		Created:     time.Now(),
	}

	var feedItems []*feeds.Item

	// Convert Markdown links to HTML for RSS feed
	contentHTML := util.MarkdownLinksToHTML(note.Message)

	feedItems = append(feedItems,
		&feeds.Item{
			Id:      note.Id.String(),
			Title:   note.CreatedAt.Format(util.DateTimeFormat()),
			Link:    &feeds.Link{Href: url},
			Content: contentHTML,
			Author:  &feeds.Author{Name: note.CreatedBy, Email: email},
			Created: note.CreatedAt,
		})

	feed.Items = feedItems
	return feed.ToRss()
}
