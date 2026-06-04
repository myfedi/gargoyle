package activitypub

import (
	"context"
	"strings"

	"github.com/myfedi/gargoyle/domain/ports/db"
	"github.com/myfedi/gargoyle/domain/ports/repos"
)

func (u *HandleInboxActivityUseCase) createPollOptions(ctx context.Context, tx *db.Tx, noteID string, note ExtractedNote) error {
	return createPollOptions(ctx, u.cfg.PollsRepo, tx, noteID, note)
}

func (u *CreateOutboxActivityUseCase) createPollOptions(ctx context.Context, tx *db.Tx, noteID string, note ExtractedNote) error {
	return createPollOptions(ctx, u.cfg.PollsRepo, tx, noteID, note)
}

func createPollOptions(ctx context.Context, repo repos.PollsRepository, tx *db.Tx, noteID string, note ExtractedNote) error {
	if repo == nil || note.Type != "Question" {
		return nil
	}
	options := normalizedPollOptions(note.PollOptions)
	if len(options) == 0 {
		return nil
	}
	_, err := repo.CreatePoll(ctx, tx, repos.CreatePollInput{NoteID: noteID, Options: options, Multiple: note.PollMultiple})
	return err
}

func normalizedPollOptions(raw []string) []string {
	seen := map[string]bool{}
	options := make([]string, 0, len(raw))
	for _, option := range raw {
		option = strings.TrimSpace(option)
		if option == "" || seen[strings.ToLower(option)] {
			continue
		}
		seen[strings.ToLower(option)] = true
		options = append(options, option)
	}
	return options
}
