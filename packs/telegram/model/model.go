// Package model holds the plain data types passed between fetch, summarize,
// and send. Keeping them here (with no Telegram-library imports) avoids import
// cycles and keeps the summarizer/sender testable without a live client.
package model

// ChatKind classifies a chat for digest ordering (DMs first, then groups, then
// channels — the ordering Sumit chose).
type ChatKind int

const (
	KindDM      ChatKind = iota // 1:1 private conversation
	KindGroup                   // basic group or supergroup
	KindChannel                 // broadcast channel
)

func (k ChatKind) String() string {
	switch k {
	case KindDM:
		return "DM"
	case KindGroup:
		return "Group"
	case KindChannel:
		return "Channel"
	default:
		return "Unknown"
	}
}

// Message is one fetched Telegram message, reduced to what the summarizer needs.
type Message struct {
	ID     int    // Telegram message ID (monotonic within a chat)
	Date   int64  // unix seconds
	Sender string // display name of the sender ("" for channel posts)
	// Text is the message text or media caption. For media with no caption it
	// is empty and MediaType describes the content instead.
	Text string
	// MediaType is non-empty for non-text messages: "photo", "voice", "video",
	// "sticker", "document", "poll", etc. Media is described, never transcribed.
	MediaType string
}

// Chat is one conversation with its new (since-checkpoint) messages.
type Chat struct {
	ID       int64
	Title    string   // display title (peer name or group/channel title)
	Kind     ChatKind
	Messages []Message // ascending by ID; only messages newer than the checkpoint
}

// NewestMessageID returns the highest message ID in the chat (0 if none).
func (c *Chat) NewestMessageID() int {
	max := 0
	for _, m := range c.Messages {
		if m.ID > max {
			max = m.ID
		}
	}
	return max
}
