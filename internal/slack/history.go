// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package slack

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/jacob-delgado/workflow/internal/sanitize"
)

const (
	// conversationsListPath resolves a channel name to the ID history needs.
	conversationsListPath = "/conversations.list"
	// conversationsHistoryPath reads a channel's recent messages.
	conversationsHistoryPath = "/conversations.history"
)

const (
	// channelPageLimit is how many channels a single list page asks for; 1000 is
	// the API's maximum.
	channelPageLimit = 1000
	// maxChannelPages bounds the search for the channel, so a huge workspace
	// cannot make the lookup run forever.
	maxChannelPages = 10
	// historyLimit is how many recent messages to read looking for the URL. An
	// announcement is among the most recent posts, not buried a thousand deep.
	historyLimit = 200
)

// ErrChannelNotFound reports a channel whose history could not be read because
// no channel by that name is one the bot can see.
var ErrChannelNotFound = errors.New("no such channel, or the bot cannot see it")

// AlreadyPosted reports whether link already appears in the recent history of
// channel, so a pull request announced in an earlier session is not offered for
// announcing again. It needs a bot token and the channel's history scope; a
// webhook cannot read history. An empty channel means the configured default.
func (c Client) AlreadyPosted(ctx context.Context, channel, link string) (bool, error) {
	err := c.checkable()
	if err != nil {
		return false, err
	}

	if channel == "" {
		channel = c.creds.Channel
	}

	id, err := c.channelID(ctx, channel)
	if err != nil {
		return false, err
	}

	return c.historyHas(ctx, id, link)
}

// channelID resolves a channel name, as configured with or without a leading
// "#", to the ID conversations.history takes. A value that is already an ID is
// matched too, so an ID in the configuration works without a lookup miss.
func (c Client) channelID(ctx context.Context, channel string) (string, error) {
	name := strings.TrimPrefix(channel, "#")

	cursor := ""
	for range maxChannelPages {
		listed, err := c.listChannels(ctx, cursor)
		if err != nil {
			return "", err
		}

		for _, conversation := range listed.Channels {
			if conversation.Name == name || conversation.ID == channel {
				return conversation.ID, nil
			}
		}

		cursor = listed.Metadata.NextCursor
		if cursor == "" {
			break
		}
	}

	return "", fmt.Errorf("%w: %s", ErrChannelNotFound, channel)
}

// channelList is one page of conversations.list.
type channelList struct {
	OK       bool      `json:"ok"`
	Error    string    `json:"error"`
	Channels []channel `json:"channels"`
	Metadata struct {
		NextCursor string `json:"next_cursor"`
	} `json:"response_metadata"`
}

// channel is a conversation the bot can see, by ID and name.
type channel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// listChannels reads one page of the channels the bot can see, from cursor.
func (c Client) listChannels(ctx context.Context, cursor string) (channelList, error) {
	query := url.Values{
		"types":            {"public_channel,private_channel"},
		"limit":            {strconv.Itoa(channelPageLimit)},
		"exclude_archived": {"true"},
	}
	if cursor != "" {
		query.Set("cursor", cursor)
	}

	var listed channelList

	err := c.readJSON(ctx, conversationsListPath, query, &listed)
	if err != nil {
		return channelList{}, err
	}

	if !listed.OK {
		return channelList{}, fmt.Errorf("%w: %s", ErrRejected, listed.Error)
	}

	return listed, nil
}

// channelHistory is conversations.history's recent messages.
type channelHistory struct {
	OK       bool   `json:"ok"`
	Error    string `json:"error"`
	Messages []struct {
		Text string `json:"text"`
	} `json:"messages"`
}

// historyHas reports whether link appears in the recent messages of the channel
// with the given ID.
func (c Client) historyHas(ctx context.Context, channelID, link string) (bool, error) {
	query := url.Values{"channel": {channelID}, "limit": {strconv.Itoa(historyLimit)}}

	var history channelHistory

	err := c.readJSON(ctx, conversationsHistoryPath, query, &history)
	if err != nil {
		return false, err
	}

	if !history.OK {
		return false, fmt.Errorf("%w: %s", ErrRejected, history.Error)
	}

	for _, message := range history.Messages {
		if strings.Contains(message.Text, link) {
			return true, nil
		}
	}

	return false, nil
}

// readJSON performs a GET against a Slack Web API method and decodes its answer
// into out. The token travels in the header, as it does for every other call.
func (c Client) readJSON(ctx context.Context, path string, query url.Values, out any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path+"?"+query.Encode(), nil)
	if err != nil {
		return fmt.Errorf("%w: building the request", ErrUnreachable)
	}

	request.Header.Set("Authorization", "Bearer "+c.creds.Token)
	request.Header.Set("Accept", "application/json")

	body, err := c.deliver(request)
	if err != nil {
		return err
	}

	err = json.Unmarshal(sanitize.JSON(body), out)
	if err != nil {
		return fmt.Errorf("reading the answer from Slack: %w", err)
	}

	return nil
}
