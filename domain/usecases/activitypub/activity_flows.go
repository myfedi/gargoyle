package activitypub

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
	"github.com/myfedi/gargoyle/domain/ports/db"
	"github.com/myfedi/gargoyle/domain/ports/repos"
	"github.com/myfedi/gargoyle/utils"
)

type PaginationInput struct {
	Limit  int
	Offset int
}

type ActivityPubFlowConfig struct {
	TxProvider     db.TxProvider
	AccountsRepo   repos.AccountsRepo
	ActivitiesRepo repos.ActivitiesRepository
	FollowsRepo    repos.FollowsRepository
	NotesRepo      repos.NotesRepository
}

type GetOutboxUseCase struct{ cfg ActivityPubFlowConfig }
type GetFollowersUseCase struct{ cfg ActivityPubFlowConfig }
type GetFollowingUseCase struct{ cfg ActivityPubFlowConfig }
type CreateFollowingUseCase struct{ cfg ActivityPubFlowConfig }
type CreateOutboxActivityUseCase struct{ cfg ActivityPubFlowConfig }
type HandleInboxActivityUseCase struct{ cfg ActivityPubFlowConfig }

func NewGetOutboxUseCase(cfg ActivityPubFlowConfig) GetOutboxUseCase {
	return GetOutboxUseCase{cfg: cfg}
}
func NewGetFollowersUseCase(cfg ActivityPubFlowConfig) GetFollowersUseCase {
	return GetFollowersUseCase{cfg: cfg}
}
func NewGetFollowingUseCase(cfg ActivityPubFlowConfig) GetFollowingUseCase {
	return GetFollowingUseCase{cfg: cfg}
}
func NewCreateFollowingUseCase(cfg ActivityPubFlowConfig) CreateFollowingUseCase {
	return CreateFollowingUseCase{cfg: cfg}
}
func NewCreateOutboxActivityUseCase(cfg ActivityPubFlowConfig) CreateOutboxActivityUseCase {
	return CreateOutboxActivityUseCase{cfg: cfg}
}
func NewHandleInboxActivityUseCase(cfg ActivityPubFlowConfig) HandleInboxActivityUseCase {
	return HandleInboxActivityUseCase{cfg: cfg}
}

type OutboxResult struct {
	Account    models.Account
	Activities []models.Activity
	Total      int
}

type FollowersResult struct {
	Account   models.Account
	Followers []models.Follow
	Total     int
}

type FollowingResult struct {
	Account   models.Account
	Following []models.Follow
}

func (u *GetOutboxUseCase) GetOutbox(ctx context.Context, username string, page PaginationInput) (*OutboxResult, *domainerrors.DomainError) {
	account, derr := localAccount(ctx, u.cfg.AccountsRepo, username)
	if derr != nil {
		return nil, derr
	}
	activities, err := u.cfg.ActivitiesRepo.ListOutboxActivitiesPaged(ctx, nil, account.ID, page.Limit, page.Offset)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	total, err := u.cfg.ActivitiesRepo.CountOutboxActivities(ctx, nil, account.ID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return &OutboxResult{Account: *account, Activities: activities, Total: total}, nil
}

func (u *GetFollowersUseCase) GetFollowers(ctx context.Context, username string, page PaginationInput) (*FollowersResult, *domainerrors.DomainError) {
	account, derr := localAccount(ctx, u.cfg.AccountsRepo, username)
	if derr != nil {
		return nil, derr
	}
	followers, err := u.cfg.FollowsRepo.ListFollowersPaged(ctx, nil, account.ID, page.Limit, page.Offset)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	total, err := u.cfg.FollowsRepo.CountFollowers(ctx, nil, account.ID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return &FollowersResult{Account: *account, Followers: followers, Total: total}, nil
}

func (u *GetFollowingUseCase) GetFollowing(ctx context.Context, username string) (*FollowingResult, *domainerrors.DomainError) {
	account, derr := localAccount(ctx, u.cfg.AccountsRepo, username)
	if derr != nil {
		return nil, derr
	}
	following, err := u.cfg.FollowsRepo.ListFollowing(ctx, nil, account.ID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return &FollowingResult{Account: *account, Following: following}, nil
}

type CreateFollowingInput struct {
	Username string
	Actor    string
	Inbox    string
	FollowID string
}

type CreateFollowingResult struct {
	Account models.Account
	RawJSON []byte
	Inbox   string
}

func (u *CreateFollowingUseCase) GetLocalAccount(ctx context.Context, username string) (*models.Account, *domainerrors.DomainError) {
	return localAccount(ctx, u.cfg.AccountsRepo, username)
}

func (u *CreateFollowingUseCase) CreateFollowing(ctx context.Context, input CreateFollowingInput) (*CreateFollowingResult, *domainerrors.DomainError) {
	account, derr := localAccount(ctx, u.cfg.AccountsRepo, input.Username)
	if derr != nil {
		return nil, derr
	}
	if input.Actor == "" {
		return nil, domainerrors.New(domainerrors.ErrBadRequest, "actor is required")
	}
	if input.FollowID == "" {
		return nil, domainerrors.New(domainerrors.ErrInternal, "follow id is required")
	}

	followActivity := map[string]any{"@context": "https://www.w3.org/ns/activitystreams", "id": account.URI + "/follows/" + input.FollowID, "type": "Follow", "actor": account.URI, "object": input.Actor}
	raw, err := json.Marshal(followActivity)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}

	err = u.cfg.TxProvider.RunInTx(ctx, sql.TxOptions{}, func(ctx context.Context, tx db.Tx) error {
		stored, err := u.cfg.ActivitiesRepo.CreateActivity(ctx, &tx, repos.CreateActivityInput{LocalAccountID: account.ID, Direction: models.ActivityDirectionOutbox, Type: "Follow", Actor: account.URI, Object: input.Actor, RawJSON: string(raw)})
		if err != nil {
			return err
		}
		var inboxPtr *string
		if input.Inbox != "" {
			inboxPtr = &input.Inbox
		}
		_, err = u.cfg.FollowsRepo.CreateFollowing(ctx, &tx, repos.CreateFollowInput{LocalAccountID: account.ID, RemoteActor: input.Actor, RemoteInbox: inboxPtr, ActivityID: stored.ID})
		return err
	})
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return &CreateFollowingResult{Account: *account, RawJSON: raw, Inbox: input.Inbox}, nil
}

type CreateOutboxActivityInput struct {
	Username   string
	RawJSON    []byte
	ActivityID string
	ObjectID   string
}

type CreateOutboxActivityResult struct {
	Account         models.Account
	RawJSON         []byte
	FollowerInboxes []string
}

func (u *CreateOutboxActivityUseCase) CreateOutboxActivity(ctx context.Context, input CreateOutboxActivityInput) (*CreateOutboxActivityResult, *domainerrors.DomainError) {
	account, derr := localAccount(ctx, u.cfg.AccountsRepo, input.Username)
	if derr != nil {
		return nil, derr
	}
	raw, derr := NormalizeOutboxActivity(input.RawJSON, *account, input.ActivityID, input.ObjectID)
	if derr != nil {
		return nil, derr
	}
	activity, derr := ParseActivity(raw)
	if derr != nil {
		return nil, derr
	}
	if activity.Actor != "" && activity.Actor != account.URI {
		return nil, domainerrors.New(domainerrors.ErrBadRequest, "activity actor does not match local actor")
	}

	err := u.cfg.TxProvider.RunInTx(ctx, sql.TxOptions{}, func(ctx context.Context, tx db.Tx) error {
		stored, err := u.cfg.ActivitiesRepo.CreateActivity(ctx, &tx, repos.CreateActivityInput{LocalAccountID: account.ID, Direction: models.ActivityDirectionOutbox, Type: activity.Type, Actor: account.URI, Object: activity.Object, RawJSON: string(raw)})
		if err != nil {
			return err
		}
		if u.cfg.NotesRepo != nil {
			if note, ok := ExtractNote(raw); ok {
				_, err := u.cfg.NotesRepo.CreateNote(ctx, &tx, repos.CreateNoteInput{LocalAccountID: account.ID, ActivityID: stored.ID, URI: note.URI, Content: utils.SanitizeHTML(note.Content), PlainText: utils.StripHTMLFromText(note.Content), AttributedTo: note.AttributedTo, PublishedAt: note.PublishedAt})
				if err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}

	var inboxes []string
	if u.cfg.FollowsRepo != nil {
		followers, err := u.cfg.FollowsRepo.ListFollowers(ctx, nil, account.ID)
		if err != nil {
			return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
		}
		for _, follower := range followers {
			if follower.RemoteInbox != nil {
				inboxes = append(inboxes, *follower.RemoteInbox)
			}
		}
	}
	return &CreateOutboxActivityResult{Account: *account, RawJSON: raw, FollowerInboxes: inboxes}, nil
}

type HandleInboxActivityInput struct {
	Username string
	RawJSON  []byte
	Inbox    string
}

type HandleInboxActivityResult struct {
	Account     models.Account
	Activity    ParsedActivity
	AcceptJSON  []byte
	AcceptInbox string
}

func (u *HandleInboxActivityUseCase) InspectInboxActivity(ctx context.Context, username string, raw []byte) (*models.Account, ParsedActivity, *domainerrors.DomainError) {
	account, derr := localAccount(ctx, u.cfg.AccountsRepo, username)
	if derr != nil {
		return nil, ParsedActivity{}, derr
	}
	activity, derr := ParseActivity(raw)
	if derr != nil {
		return nil, ParsedActivity{}, derr
	}
	return account, activity, nil
}

func (u *HandleInboxActivityUseCase) HandleInboxActivity(ctx context.Context, input HandleInboxActivityInput) (*HandleInboxActivityResult, *domainerrors.DomainError) {
	account, activity, derr := u.InspectInboxActivity(ctx, input.Username, input.RawJSON)
	if derr != nil {
		return nil, derr
	}

	// Validate before writing when possible.
	if activity.Type == "Follow" && activity.Object != account.URI {
		return nil, domainerrors.New(domainerrors.ErrBadRequest, "follow object does not match local actor")
	}

	var acceptJSON []byte
	acceptInbox := input.Inbox
	err := u.cfg.TxProvider.RunInTx(ctx, sql.TxOptions{}, func(ctx context.Context, tx db.Tx) error {
		stored, err := u.cfg.ActivitiesRepo.CreateActivity(ctx, &tx, repos.CreateActivityInput{LocalAccountID: account.ID, Direction: models.ActivityDirectionInbox, Type: activity.Type, Actor: activity.Actor, Object: activity.Object, RawJSON: string(input.RawJSON)})
		if err != nil {
			return err
		}

		switch activity.Type {
		case "Follow":
			if u.cfg.FollowsRepo == nil {
				return nil
			}
			var inboxPtr *string
			if acceptInbox != "" {
				inboxPtr = &acceptInbox
			}
			follow, err := u.cfg.FollowsRepo.CreateFollow(ctx, &tx, repos.CreateFollowInput{LocalAccountID: account.ID, RemoteActor: activity.Actor, RemoteInbox: inboxPtr, ActivityID: stored.ID})
			if err != nil {
				return err
			}
			if err := u.cfg.FollowsRepo.AcceptFollow(ctx, &tx, follow.ID); err != nil {
				return err
			}
			acceptJSON, err = MarshalAccept(*account, *follow, input.RawJSON)
			return err
		case "Create":
			if u.cfg.NotesRepo != nil {
				if note, ok := ExtractNote(input.RawJSON); ok {
					_, err := u.cfg.NotesRepo.CreateNote(ctx, &tx, repos.CreateNoteInput{LocalAccountID: account.ID, ActivityID: stored.ID, URI: note.URI, Content: utils.SanitizeHTML(note.Content), PlainText: utils.StripHTMLFromText(note.Content), AttributedTo: note.AttributedTo, PublishedAt: note.PublishedAt})
					return err
				}
			}
		case "Delete":
			if u.cfg.NotesRepo != nil && activity.Object != "" {
				return u.cfg.NotesRepo.DeleteNoteByURI(ctx, &tx, activity.Object)
			}
		case "Update":
			if u.cfg.NotesRepo != nil {
				if note, ok := ExtractNoteObject(input.RawJSON); ok {
					return u.cfg.NotesRepo.UpdateNoteByURI(ctx, &tx, note.URI, utils.SanitizeHTML(note.Content), utils.StripHTMLFromText(note.Content))
				}
			}
		case "Accept":
			if u.cfg.FollowsRepo != nil && activity.Actor != "" {
				return u.cfg.FollowsRepo.AcceptFollowingByActor(ctx, &tx, account.ID, activity.Actor)
			}
		case "Reject":
			if u.cfg.FollowsRepo != nil && activity.Actor != "" {
				return u.cfg.FollowsRepo.RejectFollowingByActor(ctx, &tx, account.ID, activity.Actor)
			}
		case "Undo":
			if u.cfg.FollowsRepo != nil {
				remoteActor, err := ExtractUndoFollowActor(input.RawJSON)
				if err != nil {
					return domainerrors.NewErr(domainerrors.ErrBadRequest, err)
				}
				if remoteActor != "" {
					return u.cfg.FollowsRepo.DeleteFollowByActor(ctx, &tx, account.ID, remoteActor)
				}
			}
		}
		return nil
	})
	if err != nil {
		var derr *domainerrors.DomainError
		if errors.As(err, &derr) {
			return nil, derr
		}
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return &HandleInboxActivityResult{Account: *account, Activity: activity, AcceptJSON: acceptJSON, AcceptInbox: acceptInbox}, nil
}

func localAccount(ctx context.Context, repo repos.AccountsRepo, username string) (*models.Account, *domainerrors.DomainError) {
	if username == "" {
		return nil, domainerrors.New(domainerrors.ErrBadRequest, "missing username")
	}
	account, err := repo.GetLocalAccountByUsername(ctx, nil, username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domainerrors.New(domainerrors.ErrNotFound, "no such username")
		}
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return account, nil
}

type ParsedActivity struct {
	Type   string
	Actor  string
	Object string
	Inbox  string
}

func ParseActivity(raw []byte) (ParsedActivity, *domainerrors.DomainError) {
	var envelope struct {
		Context json.RawMessage `json:"@context,omitempty"`
		ID      string          `json:"id,omitempty"`
		Type    string          `json:"type"`
		Actor   json.RawMessage `json:"actor"`
		Object  json.RawMessage `json:"object,omitempty"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return ParsedActivity{}, domainerrors.NewErr(domainerrors.ErrBadRequest, err)
	}
	if envelope.Type == "" {
		return ParsedActivity{}, domainerrors.New(domainerrors.ErrBadRequest, "activity type is required")
	}
	actor, inbox, err := ExtractIDAndInbox(envelope.Actor)
	if err != nil {
		return ParsedActivity{}, domainerrors.NewErr(domainerrors.ErrBadRequest, fmt.Errorf("invalid actor: %w", err))
	}
	object, _, err := ExtractIDAndInbox(envelope.Object)
	if len(envelope.Object) > 0 && err != nil {
		return ParsedActivity{}, domainerrors.NewErr(domainerrors.ErrBadRequest, fmt.Errorf("invalid object: %w", err))
	}
	return ParsedActivity{Type: envelope.Type, Actor: actor, Object: object, Inbox: inbox}, nil
}

func NormalizeOutboxActivity(raw []byte, account models.Account, activityID string, objectID string) ([]byte, *domainerrors.DomainError) {
	if activityID == "" || objectID == "" {
		return nil, domainerrors.New(domainerrors.ErrInternal, "activity and object ids are required")
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrBadRequest, err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	typeValue, _ := doc["type"].(string)
	if typeValue == "" {
		if _, ok := doc["content"]; ok {
			typeValue = "Note"
			doc["type"] = typeValue
		} else {
			return nil, domainerrors.New(domainerrors.ErrBadRequest, "activity type is required")
		}
	}
	if typeValue != "Create" {
		object := doc
		SanitizeObjectContent(object)
		if _, ok := object["@context"]; !ok {
			object["@context"] = "https://www.w3.org/ns/activitystreams"
		}
		if _, ok := object["id"]; !ok {
			object["id"] = account.URI + "/objects/" + objectID
		}
		if _, ok := object["attributedTo"]; !ok {
			object["attributedTo"] = account.URI
		}
		if _, ok := object["published"]; !ok {
			object["published"] = now
		}
		if _, ok := object["to"]; !ok {
			object["to"] = []string{"https://www.w3.org/ns/activitystreams#Public"}
		}
		if _, ok := object["cc"]; !ok {
			object["cc"] = []string{account.FollowersURI}
		}
		doc = map[string]any{"@context": "https://www.w3.org/ns/activitystreams", "id": account.URI + "/activities/" + activityID, "type": "Create", "actor": account.URI, "published": now, "to": object["to"], "cc": object["cc"], "object": object}
	} else {
		if object, ok := doc["object"].(map[string]any); ok {
			SanitizeObjectContent(object)
		}
		if _, ok := doc["@context"]; !ok {
			doc["@context"] = "https://www.w3.org/ns/activitystreams"
		}
		if _, ok := doc["id"]; !ok {
			doc["id"] = account.URI + "/activities/" + activityID
		}
		doc["actor"] = account.URI
		if _, ok := doc["published"]; !ok {
			doc["published"] = now
		}
		if _, ok := doc["to"]; !ok {
			doc["to"] = []string{"https://www.w3.org/ns/activitystreams#Public"}
		}
		if _, ok := doc["cc"]; !ok {
			doc["cc"] = []string{account.FollowersURI}
		}
		if object, ok := doc["object"].(map[string]any); ok {
			if _, ok := object["to"]; !ok {
				object["to"] = doc["to"]
			}
			if _, ok := object["cc"]; !ok {
				object["cc"] = doc["cc"]
			}
		}
	}
	res, err := json.Marshal(doc)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return res, nil
}

func SanitizeObjectContent(object map[string]any) {
	if typ, _ := object["type"].(string); typ != "" && typ != "Note" {
		return
	}
	if content, ok := object["content"].(string); ok {
		object["content"] = utils.SanitizeHTML(content)
	}
}

type ExtractedNote struct {
	URI          string
	Content      string
	AttributedTo string
	PublishedAt  time.Time
}

func ExtractNote(raw []byte) (ExtractedNote, bool) {
	var activity struct {
		Type   string          `json:"type"`
		Object json.RawMessage `json:"object"`
	}
	if err := json.Unmarshal(raw, &activity); err != nil || activity.Type != "Create" || len(activity.Object) == 0 {
		return ExtractedNote{}, false
	}
	var note struct {
		ID           string `json:"id"`
		Type         string `json:"type"`
		Content      string `json:"content"`
		AttributedTo string `json:"attributedTo"`
		Published    string `json:"published"`
	}
	if err := json.Unmarshal(activity.Object, &note); err != nil || note.Type != "Note" || note.ID == "" {
		return ExtractedNote{}, false
	}
	publishedAt, err := time.Parse(time.RFC3339, note.Published)
	if err != nil {
		publishedAt = time.Now().UTC()
	}
	return ExtractedNote{URI: note.ID, Content: note.Content, AttributedTo: note.AttributedTo, PublishedAt: publishedAt}, true
}

func ExtractNoteObject(raw []byte) (ExtractedNote, bool) {
	var activity struct {
		Object json.RawMessage `json:"object"`
	}
	if err := json.Unmarshal(raw, &activity); err != nil || len(activity.Object) == 0 {
		return ExtractedNote{}, false
	}
	var note struct {
		ID           string `json:"id"`
		Type         string `json:"type"`
		Content      string `json:"content"`
		AttributedTo string `json:"attributedTo"`
		Published    string `json:"published"`
	}
	if err := json.Unmarshal(activity.Object, &note); err != nil || note.Type != "Note" || note.ID == "" {
		return ExtractedNote{}, false
	}
	publishedAt, err := time.Parse(time.RFC3339, note.Published)
	if err != nil {
		publishedAt = time.Now().UTC()
	}
	return ExtractedNote{URI: note.ID, Content: note.Content, AttributedTo: note.AttributedTo, PublishedAt: publishedAt}, true
}

func ExtractUndoFollowActor(raw []byte) (string, error) {
	var doc struct {
		Object json.RawMessage `json:"object"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return "", err
	}
	var obj struct {
		Type  string          `json:"type"`
		Actor json.RawMessage `json:"actor"`
	}
	if err := json.Unmarshal(doc.Object, &obj); err != nil {
		actor, _, actorErr := ExtractIDAndInbox(doc.Object)
		if actorErr != nil {
			return "", err
		}
		return actor, nil
	}
	if obj.Type != "Follow" {
		return "", nil
	}
	actor, _, err := ExtractIDAndInbox(obj.Actor)
	return actor, err
}

func ExtractIDAndInbox(raw json.RawMessage) (string, string, error) {
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return "", "", nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s, "", nil
	}
	var obj struct {
		ID    string `json:"id"`
		Inbox string `json:"inbox"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return "", "", err
	}
	return obj.ID, obj.Inbox, nil
}

func MarshalAccept(account models.Account, follow models.Follow, followRaw []byte) ([]byte, error) {
	accept := map[string]any{"@context": "https://www.w3.org/ns/activitystreams", "id": account.URI + "/accepts/" + follow.ID, "type": "Accept", "actor": account.URI, "object": json.RawMessage(followRaw)}
	return json.Marshal(accept)
}
