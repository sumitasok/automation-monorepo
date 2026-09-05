// Package fetch reads new messages from every visible chat via the raw MTProto
// API. It is strictly read-only: the only calls it makes are messages.getDialogs
// and messages.getHistory. Per-chat it fetches only messages newer than the
// stored checkpoint (min_id), so a run never re-summarizes old content.
package fetch

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/gotd/td/tg"

	"github.com/sumitasok/sa.automation.telegram/config"
	"github.com/sumitasok/sa.automation.telegram/model"
)

const (
	dialogBatchLimit = 100 // dialogs per messages.getDialogs page
	maxDialogBatches = 10  // hard cap: scan at most 1000 most-recent dialogs
	historyLimit     = 100 // messages per messages.getHistory page
	maxHistoryPages  = 10  // hard cap: at most 1000 new messages per chat per run
)

// Result is what FetchNewMessages returns: the active chats (each with its new
// messages) plus a count of messages gathered.
type Result struct {
	Chats     []model.Chat
	TotalMsgs int
}

// FetchNewMessages walks the user's dialogs (most-recent first) and collects,
// per chat, the messages newer than the checkpoint in state. On a chat's first
// ever run (no checkpoint) it bounds the fetch to cfg.FirstRunLookbackHours.
func FetchNewMessages(ctx context.Context, api *tg.Client, cfg *config.Config, state *config.State) (*Result, error) {
	firstRunCutoff := time.Now().Add(-time.Duration(cfg.FirstRunLookbackHours) * time.Hour).Unix()

	res := &Result{}

	var (
		offsetDate int
		offsetID   int
		offsetPeer tg.InputPeerClass = &tg.InputPeerEmpty{}
	)

	for batch := 0; batch < maxDialogBatches; batch++ {
		resp, err := api.MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{
			OffsetDate: offsetDate,
			OffsetID:   offsetID,
			OffsetPeer: offsetPeer,
			Limit:      dialogBatchLimit,
			Hash:       0,
		})
		if err != nil {
			return nil, fmt.Errorf("messages.getDialogs: %w", err)
		}

		dialogs, messages, chats, users := unpackDialogs(resp)
		if len(dialogs) == 0 {
			break
		}
		ents := buildEntities(chats, users)

		batchHadNew := false
		for _, dc := range dialogs {
			d, ok := dc.(*tg.Dialog)
			if !ok {
				continue
			}
			peer, ok := ents.resolve(d.Peer)
			if !ok {
				continue
			}
			if cfg.ExcludeChatIDs[peer.chatID] {
				continue
			}

			checkpoint := state.Checkpoint(peer.chatID)
			// TopMessage is the newest message ID in the dialog. If it's not
			// past our checkpoint, the chat has nothing new — skip the history
			// call entirely.
			if d.TopMessage <= checkpoint {
				continue
			}
			batchHadNew = true

			msgs, err := fetchChatHistory(ctx, api, peer.input, checkpoint, firstRunCutoff)
			if err != nil {
				log.Printf("[WARN] history for %q (id=%d): %v — skipping", peer.title, peer.chatID, err)
				continue
			}
			if len(msgs) == 0 {
				continue
			}
			res.Chats = append(res.Chats, model.Chat{
				ID:       peer.chatID,
				Title:    peer.title,
				Kind:     peer.kind,
				Messages: msgs,
			})
			res.TotalMsgs += len(msgs)
		}

		// Dialogs are ordered by most-recent activity. Once an entire batch has
		// no chat with new messages, everything further down is older still.
		if !batchHadNew {
			break
		}
		if len(dialogs) < dialogBatchLimit {
			break // last page
		}

		// Advance the offset to the oldest dialog in this batch.
		last, ok := dialogs[len(dialogs)-1].(*tg.Dialog)
		if !ok {
			break
		}
		lp, ok := ents.resolve(last.Peer)
		if !ok {
			break
		}
		offsetID = last.TopMessage
		offsetDate = topMessageDate(messages, last.Peer, last.TopMessage)
		offsetPeer = lp.input
		if offsetDate == 0 {
			break // can't page reliably without a date
		}
	}

	return res, nil
}

// PeerInfo is a resolved dialog peer, used for destination resolution.
type PeerInfo struct {
	ChatID int64
	Input  tg.InputPeerClass
	Title  string
	Kind   model.ChatKind
}

// ScanPeers walks dialog pages and returns every resolvable peer. Read-only;
// used by the sender to turn a configured chat_id into an InputPeer (with the
// access hash the dialogs carry).
func ScanPeers(ctx context.Context, api *tg.Client) ([]PeerInfo, error) {
	var (
		out        []PeerInfo
		offsetDate int
		offsetID   int
		offsetPeer tg.InputPeerClass = &tg.InputPeerEmpty{}
	)
	for batch := 0; batch < maxDialogBatches; batch++ {
		resp, err := api.MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{
			OffsetDate: offsetDate,
			OffsetID:   offsetID,
			OffsetPeer: offsetPeer,
			Limit:      dialogBatchLimit,
			Hash:       0,
		})
		if err != nil {
			return nil, fmt.Errorf("messages.getDialogs: %w", err)
		}
		dialogs, messages, chats, users := unpackDialogs(resp)
		if len(dialogs) == 0 {
			break
		}
		ents := buildEntities(chats, users)
		for _, dc := range dialogs {
			d, ok := dc.(*tg.Dialog)
			if !ok {
				continue
			}
			if p, ok := ents.resolve(d.Peer); ok {
				out = append(out, PeerInfo{ChatID: p.chatID, Input: p.input, Title: p.title, Kind: p.kind})
			}
		}
		if len(dialogs) < dialogBatchLimit {
			break
		}
		last, ok := dialogs[len(dialogs)-1].(*tg.Dialog)
		if !ok {
			break
		}
		lp, ok := ents.resolve(last.Peer)
		if !ok {
			break
		}
		offsetID = last.TopMessage
		offsetDate = topMessageDate(messages, last.Peer, last.TopMessage)
		offsetPeer = lp.input
		if offsetDate == 0 {
			break
		}
	}
	return out, nil
}

// fetchChatHistory pages messages.getHistory for one peer, newest→oldest,
// collecting messages with ID > checkpoint (and, on first run, date >= cutoff).
// Returns them ascending by ID.
func fetchChatHistory(ctx context.Context, api *tg.Client, peer tg.InputPeerClass, checkpoint int, firstRunCutoff int64) ([]model.Message, error) {
	firstRun := checkpoint == 0

	var collected []model.Message
	offsetID := 0

	for page := 0; page < maxHistoryPages; page++ {
		resp, err := api.MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
			Peer:     peer,
			OffsetID: offsetID,
			Limit:    historyLimit,
			MinID:    checkpoint, // server-side: only messages with ID > checkpoint
			Hash:     0,
		})
		if err != nil {
			return nil, err
		}

		rawMsgs, users := unpackMessages(resp)
		if len(rawMsgs) == 0 {
			break
		}
		senders := senderNames(users)

		stop := false
		oldestID := 0
		for _, mc := range rawMsgs {
			m, ok := mc.(*tg.Message)
			if !ok {
				// Service/empty messages (joins, pins, etc.) carry no
				// digestible content — skip them.
				continue
			}
			if oldestID == 0 || m.ID < oldestID {
				oldestID = m.ID
			}
			if m.ID <= checkpoint {
				stop = true
				continue
			}
			if firstRun && int64(m.Date) < firstRunCutoff {
				stop = true
				continue
			}
			collected = append(collected, toModelMessage(m, senders))
		}

		if stop || len(rawMsgs) < historyLimit || oldestID == 0 {
			break
		}
		offsetID = oldestID // next page: older than the oldest we've seen
	}

	// Ascending by ID (chronological).
	for i, j := 0, len(collected)-1; i < j; i, j = i+1, j-1 {
		collected[i], collected[j] = collected[j], collected[i]
	}
	return collected, nil
}

// toModelMessage reduces a raw *tg.Message to the digest model. Media is
// described by type, never transcribed; captions/text are kept.
func toModelMessage(m *tg.Message, senders map[int64]string) model.Message {
	out := model.Message{
		ID:   m.ID,
		Date: int64(m.Date),
		Text: m.Message,
	}
	if from, ok := m.GetFromID(); ok {
		if pu, ok := from.(*tg.PeerUser); ok {
			out.Sender = senders[pu.UserID]
		}
	}
	if media, ok := m.GetMedia(); ok {
		out.MediaType = mediaType(media)
	}
	return out
}

// mediaType maps a media object to a short descriptor for the summary.
func mediaType(media tg.MessageMediaClass) string {
	switch mm := media.(type) {
	case *tg.MessageMediaPhoto:
		return "photo"
	case *tg.MessageMediaGeo, *tg.MessageMediaGeoLive:
		return "location"
	case *tg.MessageMediaContact:
		return "contact"
	case *tg.MessageMediaPoll:
		return "poll"
	case *tg.MessageMediaWebPage:
		return "" // link preview — the text already carries the URL
	case *tg.MessageMediaDocument:
		doc, ok := mm.Document.AsNotEmpty()
		if !ok {
			return "file"
		}
		return documentKind(doc)
	default:
		return "media"
	}
}

func documentKind(doc *tg.Document) string {
	isVideo := false
	for _, attr := range doc.Attributes {
		switch a := attr.(type) {
		case *tg.DocumentAttributeSticker:
			return "sticker"
		case *tg.DocumentAttributeAnimated:
			return "gif"
		case *tg.DocumentAttributeAudio:
			if a.Voice {
				return "voice note"
			}
			return "audio"
		case *tg.DocumentAttributeVideo:
			isVideo = true
		}
	}
	if isVideo {
		return "video"
	}
	return "file"
}
