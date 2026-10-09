// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package directory_test

import (
	"errors"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/messaging/directory"
)

func TestChannelMembersSaysWhichReadFailed(t *testing.T) {
	t.Parallel()

	// Each read a channel's members take, refused for want of its scope.
	cases := map[string]struct {
		path, needed string
	}{
		"the channel's lookup":  {path: "/users.conversations", needed: "channels:read"},
		"the channel's members": {path: "/conversations.members", needed: "groups:read"},
		"the workspace's users": {path: "/users.list", needed: "users:read"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			bodies := directoryBodies()
			bodies[tt.path] = `{"ok":false,"error":"missing_scope","needed":"` + tt.needed + `"}`
			_, client := startSlack(t, bodies)
			slackDirectory, _ := directoryOver(client)

			// Act
			members, err := slackDirectory.ChannelMembers(t.Context(), "#dev")

			// Assert
			missing, isMissing := errors.AsType[*messaging.MissingScopeError](err)
			if !isMissing || missing.Needed != tt.needed || members != nil {
				t.Errorf("ChannelMembers = %v, %v; want none and the %s scope named", members, err, tt.needed)
			}
		})
	}
}

func TestChannelMembersWithNoUserTokenAsksNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	slack, client := startSlack(t, directoryBodies())
	settings := &switchable{client: client, available: false}
	slackDirectory := directory.New(settings.current, time.Now)

	// Act
	members, err := slackDirectory.ChannelMembers(t.Context(), "#dev")

	// Assert
	if !errors.Is(err, messaging.ErrNoCredential) || members != nil || slack.requests() != 0 {
		t.Errorf("ChannelMembers = %v, %v after %d requests; want ErrNoCredential and nothing asked",
			members, err, slack.requests())
	}
}
