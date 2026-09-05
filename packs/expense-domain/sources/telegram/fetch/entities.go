package fetch

import (
	"fmt"
	"strings"

	"github.com/gotd/td/tg"

	"github.com/sumitasok/sa.automation.telegram/model"
)

// entities indexes the chats and users returned alongside a dialogs page so a
// tg.PeerClass can be resolved to an InputPeer (with access hash), a title, and
// a ChatKind.
type entities struct {
	users    map[int64]*tg.User
	chats    map[int64]*tg.Chat    // basic groups
	channels map[int64]*tg.Channel // channels + supergroups
}

func buildEntities(chats []tg.ChatClass, users []tg.UserClass) *entities {
	e := &entities{
		users:    map[int64]*tg.User{},
		chats:    map[int64]*tg.Chat{},
		channels: map[int64]*tg.Channel{},
	}
	for _, uc := range users {
		if u, ok := uc.(*tg.User); ok {
			e.users[u.ID] = u
		}
	}
	for _, cc := range chats {
		switch c := cc.(type) {
		case *tg.Chat:
			e.chats[c.ID] = c
		case *tg.Channel:
			e.channels[c.ID] = c
		}
	}
	return e
}

// resolvedPeer is a dialog peer resolved to everything the fetcher needs.
type resolvedPeer struct {
	input  tg.InputPeerClass
	title  string
	kind   model.ChatKind
	chatID int64
}

// resolve maps a dialog's peer to a resolvedPeer using the indexed entities.
// Returns ok=false when the referenced entity is missing or forbidden.
func (e *entities) resolve(peer tg.PeerClass) (resolvedPeer, bool) {
	switch p := peer.(type) {
	case *tg.PeerUser:
		u, ok := e.users[p.UserID]
		if !ok {
			return resolvedPeer{}, false
		}
		return resolvedPeer{
			input:  &tg.InputPeerUser{UserID: u.ID, AccessHash: u.AccessHash},
			title:  userDisplayName(u),
			kind:   model.KindDM,
			chatID: u.ID,
		}, true

	case *tg.PeerChat:
		c, ok := e.chats[p.ChatID]
		title := fmt.Sprintf("Group %d", p.ChatID)
		if ok {
			title = c.Title
		}
		return resolvedPeer{
			input:  &tg.InputPeerChat{ChatID: p.ChatID},
			title:  title,
			kind:   model.KindGroup,
			chatID: p.ChatID,
		}, true

	case *tg.PeerChannel:
		c, ok := e.channels[p.ChannelID]
		if !ok {
			return resolvedPeer{}, false
		}
		kind := model.KindGroup // megagroup / supergroup
		if c.Broadcast {
			kind = model.KindChannel
		}
		return resolvedPeer{
			input:  &tg.InputPeerChannel{ChannelID: c.ID, AccessHash: c.AccessHash},
			title:  c.Title,
			kind:   kind,
			chatID: c.ID,
		}, true
	}
	return resolvedPeer{}, false
}

func userDisplayName(u *tg.User) string {
	if u.Deleted {
		return "Deleted Account"
	}
	name := strings.TrimSpace(u.FirstName + " " + u.LastName)
	if name != "" {
		return name
	}
	if u.Username != "" {
		return "@" + u.Username
	}
	return fmt.Sprintf("User %d", u.ID)
}

// unpackDialogs extracts the slices from any concrete dialogs response.
func unpackDialogs(resp tg.MessagesDialogsClass) (dialogs []tg.DialogClass, messages []tg.MessageClass, chats []tg.ChatClass, users []tg.UserClass) {
	switch d := resp.(type) {
	case *tg.MessagesDialogs:
		return d.Dialogs, d.Messages, d.Chats, d.Users
	case *tg.MessagesDialogsSlice:
		return d.Dialogs, d.Messages, d.Chats, d.Users
	}
	return nil, nil, nil, nil
}

// unpackMessages extracts messages + users from any concrete history response.
func unpackMessages(resp tg.MessagesMessagesClass) (messages []tg.MessageClass, users []tg.UserClass) {
	switch m := resp.(type) {
	case *tg.MessagesMessages:
		return m.Messages, m.Users
	case *tg.MessagesMessagesSlice:
		return m.Messages, m.Users
	case *tg.MessagesChannelMessages:
		return m.Messages, m.Users
	}
	return nil, nil
}

func senderNames(users []tg.UserClass) map[int64]string {
	m := map[int64]string{}
	for _, uc := range users {
		if u, ok := uc.(*tg.User); ok {
			m[u.ID] = userDisplayName(u)
		}
	}
	return m
}

// topMessageDate finds the date of the message with id topID belonging to peer.
func topMessageDate(messages []tg.MessageClass, peer tg.PeerClass, topID int) int {
	var fallback int
	for _, mc := range messages {
		m, ok := mc.(*tg.Message)
		if !ok || m.ID != topID {
			continue
		}
		if peerEqual(m.PeerID, peer) {
			return m.Date
		}
		fallback = m.Date
	}
	return fallback
}

func peerEqual(a, b tg.PeerClass) bool {
	switch pa := a.(type) {
	case *tg.PeerUser:
		pb, ok := b.(*tg.PeerUser)
		return ok && pa.UserID == pb.UserID
	case *tg.PeerChat:
		pb, ok := b.(*tg.PeerChat)
		return ok && pa.ChatID == pb.ChatID
	case *tg.PeerChannel:
		pb, ok := b.(*tg.PeerChannel)
		return ok && pa.ChannelID == pb.ChannelID
	}
	return false
}
