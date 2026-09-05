// Package auth builds the gotd/td user-account MTProto client and handles the
// one-time interactive login that produces the long-lived session file.
//
// The session file is the credential (like the gmail pack's token.json): it is
// git-ignored, produced once by `go run . login`, and reused on every run.
// Permission scope is strictly read (fetch) + send (the one digest message) —
// no library call here edits, deletes, joins, or leaves anything.
package auth

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
)

// NewClient constructs an MTProto client backed by the session file at
// sessionPath. Nothing is dialed until Run is called.
func NewClient(apiID int, apiHash, sessionPath string) *telegram.Client {
	return telegram.NewClient(apiID, apiHash, telegram.Options{
		SessionStorage: &session.FileStorage{Path: sessionPath},
	})
}

// Login runs the interactive login flow and persists the session file. Safe to
// re-run: if the session is already authorized it reports success and exits.
func Login(ctx context.Context, apiID int, apiHash, sessionPath string) error {
	client := NewClient(apiID, apiHash, sessionPath)
	return client.Run(ctx, func(ctx context.Context) error {
		status, err := client.Auth().Status(ctx)
		if err != nil {
			return fmt.Errorf("checking auth status: %w", err)
		}
		if status.Authorized {
			fmt.Printf("Already authorized. Session is valid at %s\n", sessionPath)
			return nil
		}

		flow := auth.NewFlow(termAuth{}, auth.SendCodeOptions{})
		if err := client.Auth().IfNecessary(ctx, flow); err != nil {
			return fmt.Errorf("login flow: %w", err)
		}

		self, err := client.Self(ctx)
		if err != nil {
			return fmt.Errorf("fetching own account: %w", err)
		}
		fmt.Printf("\nLogged in as %s (id=%d). Session saved to %s\n",
			strings.TrimSpace(self.FirstName+" "+self.LastName), self.ID, sessionPath)
		return nil
	})
}

// WithClient runs fn with an authorized raw API client. It never prompts: if
// the session is missing or expired it returns an error directing the user to
// run `login` (so scheduled/non-interactive runs fail loudly instead of hanging).
func WithClient(ctx context.Context, apiID int, apiHash, sessionPath string,
	fn func(ctx context.Context, api *tg.Client) error) error {
	client := NewClient(apiID, apiHash, sessionPath)
	return client.Run(ctx, func(ctx context.Context) error {
		status, err := client.Auth().Status(ctx)
		if err != nil {
			return fmt.Errorf("checking auth status: %w", err)
		}
		if !status.Authorized {
			return fmt.Errorf("not authorized — run `go run . login` once to create %s", sessionPath)
		}
		return fn(ctx, client.API())
	})
}

// termAuth implements auth.UserAuthenticator by prompting on the terminal.
// Used only by the interactive `login` command.
type termAuth struct{}

func (termAuth) Phone(_ context.Context) (string, error) {
	return prompt("Enter phone number (international format, e.g. +919812345678): ")
}

func (termAuth) Code(_ context.Context, _ *tg.AuthSentCode) (string, error) {
	return prompt("Enter the login code Telegram just sent you: ")
}

func (termAuth) Password(_ context.Context) (string, error) {
	// Note: input is echoed. This runs once, interactively, on your own machine.
	return prompt("Enter your 2FA password (leave blank if none): ")
}

func (termAuth) AcceptTermsOfService(_ context.Context, _ tg.HelpTermsOfService) error {
	// Read+send-only automation on an existing account; no ToS acceptance needed.
	return nil
}

func (termAuth) SignUp(_ context.Context) (auth.UserInfo, error) {
	return auth.UserInfo{}, errors.New("sign-up is not supported: log in with an existing Telegram account")
}

func prompt(label string) (string, error) {
	fmt.Print(label)
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}
