package server

import (
	"github.com/myfedi/gargoyle/adapters"
	"github.com/myfedi/gargoyle/adapters/repos"
	"github.com/myfedi/gargoyle/domain/ports"
	domainDB "github.com/myfedi/gargoyle/domain/ports/db"
	apUsecases "github.com/myfedi/gargoyle/domain/usecases/activitypub"
	clientapiUsecases "github.com/myfedi/gargoyle/domain/usecases/clientapi"
	infra "github.com/myfedi/gargoyle/infrastructure"
	clientapiHandlers "github.com/myfedi/gargoyle/infrastructure/web/handlers/clientapi"
)

type runtimeRepos struct {
	Accounts       *repos.AccountsRepo
	Activities     *repos.ActivitiesRepo
	Notes          *repos.NotesRepo
	Follows        *repos.FollowsRepo
	Media          *repos.MediaRepo
	Social         *repos.SocialRepo
	Boosts         *repos.BoostsRepo
	Polls          *repos.PollsRepo
	Conversations  *repos.ConversationsRepo
	Mentions       *repos.MentionsRepo
	RemoteAccounts *repos.RemoteAccountsRepo
	Moderation     *repos.ModerationRepo
}

type clientAPIWorkflowInputs struct {
	Host                  string
	Domain                string
	ServerVersion         string
	TxProvider            domainDB.TxProvider
	AccountsRepo          *repos.AccountsRepo
	ActivitiesRepo        *repos.ActivitiesRepo
	NotesRepo             *repos.NotesRepo
	FollowsRepo           *repos.FollowsRepo
	MediaRepo             *repos.MediaRepo
	MediaStorage          *adapters.LocalMediaStorage
	ContentSanitizer      ports.ContentSanitizer
	SocialRepo            *repos.SocialRepo
	BoostsRepo            *repos.BoostsRepo
	PollsRepo             *repos.PollsRepo
	ConversationsRepo     *repos.ConversationsRepo
	MentionsRepo          *repos.MentionsRepo
	RemoteAccountsRepo    *repos.RemoteAccountsRepo
	ModerationRepo        *repos.ModerationRepo
	ActivityPubFlowConfig apUsecases.ActivityPubFlowConfig
	URLExceptions         []clientapiHandlers.RemoteURLException
	ProfileCacheNotifier  clientapiUsecases.RemoteProfileCacheNotifier
}

type clientAPIWorkflowContext struct {
	in                    clientAPIWorkflowInputs
	common                clientapiUsecases.CommonConfig
	idGenerator           ports.IDGenerator
	remoteResolver        clientapiUsecases.RemoteAccountResolver
	remoteObjectFetcher   clientapiHandlers.RemoteObjectFetcher
	remoteMediaFetcher    clientapiHandlers.RemoteMediaFetcher
	hydrateRemoteObjectUC apUsecases.HydrateRemoteObjectUseCase
	createOutboxUC        apUsecases.CreateOutboxActivityUseCase
	createFollowingUC     apUsecases.CreateFollowingUseCase
	undoFollowingUC       apUsecases.UndoFollowingUseCase
	createInteractionUC   apUsecases.CreateInteractionUseCase
	undoInteractionUC     apUsecases.UndoInteractionUseCase
	deleteObjectUC        apUsecases.DeleteObjectUseCase
	updateObjectUC        apUsecases.UpdateObjectUseCase
	updateActorUC         apUsecases.UpdateActorUseCase
	followDecisionUC      apUsecases.FollowRequestDecisionUseCase
	votePollUC            apUsecases.VotePollUseCase
}

type clientAPIComponents struct {
	Instance            clientapiUsecases.Instance
	Accounts            clientapiUsecases.Accounts
	Statuses            clientapiUsecases.Statuses
	Timelines           clientapiUsecases.Timelines
	Interactions        clientapiUsecases.Interactions
	ExternalInteraction clientapiUsecases.ExternalInteraction
	Notifications       clientapiUsecases.Notifications
	Conversations       clientapiUsecases.Conversations
	Media               clientapiUsecases.Media
	Profile             clientapiUsecases.Profile
	Moderation          clientapiUsecases.Moderation
	RemoteObjectFetcher clientapiHandlers.RemoteObjectFetcher
}

func clientAPIWorkflowInputsFromRuntime(host, domain string, txProvider domainDB.TxProvider, repos runtimeRepos, mediaStorage *adapters.LocalMediaStorage, sanitizer ports.ContentSanitizer, flowCfg apUsecases.ActivityPubFlowConfig, exceptions []clientapiHandlers.RemoteURLException, profileCacheNotifier clientapiUsecases.RemoteProfileCacheNotifier) clientAPIWorkflowInputs {
	return clientAPIWorkflowInputs{
		Host:                  host,
		Domain:                domain,
		ServerVersion:         infra.ServerVersion,
		TxProvider:            txProvider,
		AccountsRepo:          repos.Accounts,
		ActivitiesRepo:        repos.Activities,
		NotesRepo:             repos.Notes,
		FollowsRepo:           repos.Follows,
		MediaRepo:             repos.Media,
		MediaStorage:          mediaStorage,
		ContentSanitizer:      sanitizer,
		SocialRepo:            repos.Social,
		BoostsRepo:            repos.Boosts,
		PollsRepo:             repos.Polls,
		ConversationsRepo:     repos.Conversations,
		MentionsRepo:          repos.Mentions,
		RemoteAccountsRepo:    repos.RemoteAccounts,
		ModerationRepo:        repos.Moderation,
		ActivityPubFlowConfig: flowCfg,
		URLExceptions:         exceptions,
		ProfileCacheNotifier:  profileCacheNotifier,
	}
}

func buildClientAPIComponents(in clientAPIWorkflowInputs) clientAPIComponents {
	ctx := newClientAPIWorkflowContext(in)
	return clientAPIComponents{
		Instance:            buildInstanceWorkflow(ctx),
		Accounts:            buildAccountsWorkflow(ctx),
		Statuses:            buildStatusesWorkflow(ctx),
		Timelines:           buildTimelinesWorkflow(ctx),
		Interactions:        buildInteractionsWorkflow(ctx),
		ExternalInteraction: buildExternalInteractionWorkflow(ctx),
		Notifications:       buildNotificationsWorkflow(ctx),
		Conversations:       buildConversationsWorkflow(ctx),
		Media:               buildMediaWorkflow(ctx),
		Profile:             buildProfileWorkflow(ctx),
		Moderation:          buildModerationWorkflow(ctx),
		RemoteObjectFetcher: ctx.remoteObjectFetcher,
	}
}

func newClientAPIWorkflowContext(in clientAPIWorkflowInputs) clientAPIWorkflowContext {
	remoteObjectFetcher := clientapiHandlers.NewRemoteObjectFetcher(nil, in.URLExceptions)
	remoteMediaFetcher := clientapiHandlers.NewRemoteMediaFetcher(nil, in.URLExceptions)
	return clientAPIWorkflowContext{
		in:                  in,
		common:              clientapiUsecases.CommonConfig{Host: in.Host, Domain: in.Domain, ServerVersion: in.ServerVersion},
		idGenerator:         adapters.NewULIDGenerator(),
		remoteResolver:      clientapiHandlers.NewRemoteAccountResolver(nil, in.URLExceptions),
		remoteObjectFetcher: remoteObjectFetcher,
		remoteMediaFetcher:  remoteMediaFetcher,
		hydrateRemoteObjectUC: apUsecases.NewHydrateRemoteObjectUseCase(apUsecases.HydrateRemoteObjectConfig{
			TxProvider:         in.TxProvider,
			Fetcher:            remoteObjectFetcher,
			ActivitiesRepo:     in.ActivitiesRepo,
			NotesRepo:          in.NotesRepo,
			MediaRepo:          in.MediaRepo,
			MediaStorage:       in.MediaStorage,
			RemoteMediaFetcher: remoteMediaFetcher,
			BoostsRepo:         in.BoostsRepo,
			RemoteAccountsRepo: in.RemoteAccountsRepo,
			Sanitizer:          in.ContentSanitizer,
		}),
		createOutboxUC:      apUsecases.NewCreateOutboxActivityUseCase(in.ActivityPubFlowConfig),
		createFollowingUC:   apUsecases.NewCreateFollowingUseCase(in.ActivityPubFlowConfig),
		undoFollowingUC:     apUsecases.NewUndoFollowingUseCase(in.ActivityPubFlowConfig),
		createInteractionUC: apUsecases.NewCreateInteractionUseCase(in.ActivityPubFlowConfig),
		undoInteractionUC:   apUsecases.NewUndoInteractionUseCase(in.ActivityPubFlowConfig),
		deleteObjectUC:      apUsecases.NewDeleteObjectUseCase(in.ActivityPubFlowConfig),
		updateObjectUC:      apUsecases.NewUpdateObjectUseCase(in.ActivityPubFlowConfig),
		updateActorUC:       apUsecases.NewUpdateActorUseCase(in.ActivityPubFlowConfig),
		followDecisionUC:    apUsecases.NewFollowRequestDecisionUseCase(in.ActivityPubFlowConfig),
		votePollUC:          apUsecases.NewVotePollUseCase(in.ActivityPubFlowConfig),
	}
}

func buildInstanceWorkflow(ctx clientAPIWorkflowContext) clientapiUsecases.Instance {
	return clientapiUsecases.NewInstance(clientapiUsecases.InstanceConfig{CommonConfig: ctx.common})
}

func buildAccountsWorkflow(ctx clientAPIWorkflowContext) clientapiUsecases.Accounts {
	in := ctx.in
	return clientapiUsecases.NewAccounts(clientapiUsecases.AccountsConfig{
		CommonConfig:          ctx.common,
		AccountsRepo:          in.AccountsRepo,
		NotesRepo:             in.NotesRepo,
		FollowsRepo:           in.FollowsRepo,
		MediaRepo:             in.MediaRepo,
		MediaStorage:          in.MediaStorage,
		RemoteMediaFetcher:    ctx.remoteMediaFetcher,
		SocialRepo:            in.SocialRepo,
		BoostsRepo:            in.BoostsRepo,
		MentionsRepo:          in.MentionsRepo,
		PollsRepo:             in.PollsRepo,
		RemoteAccountsRepo:    in.RemoteAccountsRepo,
		DomainBlocksRepo:      in.ModerationRepo,
		IDGenerator:           ctx.idGenerator,
		RemoteResolver:        ctx.remoteResolver,
		ProfileCacheNotifier:  in.ProfileCacheNotifier,
		CreateFollowingUC:     ctx.createFollowingUC,
		UndoFollowingUC:       ctx.undoFollowingUC,
		FollowDecisionUC:      ctx.followDecisionUC,
		HydrateRemoteObjectUC: ctx.hydrateRemoteObjectUC,
	})
}

func buildStatusesWorkflow(ctx clientAPIWorkflowContext) clientapiUsecases.Statuses {
	in := ctx.in
	return clientapiUsecases.NewStatuses(clientapiUsecases.StatusesConfig{
		CommonConfig:          ctx.common,
		NotesRepo:             in.NotesRepo,
		AccountsRepo:          in.AccountsRepo,
		MediaRepo:             in.MediaRepo,
		MediaStorage:          in.MediaStorage,
		SocialRepo:            in.SocialRepo,
		BoostsRepo:            in.BoostsRepo,
		MentionsRepo:          in.MentionsRepo,
		PollsRepo:             in.PollsRepo,
		DomainBlocksRepo:      in.ModerationRepo,
		RemoteAccountsRepo:    in.RemoteAccountsRepo,
		RemoteResolver:        ctx.remoteResolver,
		ContentSanitizer:      in.ContentSanitizer,
		IDGenerator:           ctx.idGenerator,
		CreateOutboxUC:        ctx.createOutboxUC,
		DeleteObjectUC:        ctx.deleteObjectUC,
		UpdateObjectUC:        ctx.updateObjectUC,
		HydrateRemoteObjectUC: ctx.hydrateRemoteObjectUC,
	})
}

func buildTimelinesWorkflow(ctx clientAPIWorkflowContext) clientapiUsecases.Timelines {
	in := ctx.in
	return clientapiUsecases.NewTimelines(clientapiUsecases.TimelinesConfig{
		CommonConfig:         ctx.common,
		NotesRepo:            in.NotesRepo,
		AccountsRepo:         in.AccountsRepo,
		FollowsRepo:          in.FollowsRepo,
		MediaRepo:            in.MediaRepo,
		MediaStorage:         in.MediaStorage,
		RemoteMediaFetcher:   ctx.remoteMediaFetcher,
		SocialRepo:           in.SocialRepo,
		BoostsRepo:           in.BoostsRepo,
		MentionsRepo:         in.MentionsRepo,
		PollsRepo:            in.PollsRepo,
		RemoteAccountsRepo:   in.RemoteAccountsRepo,
		DomainBlocksRepo:     in.ModerationRepo,
		RemoteResolver:       ctx.remoteResolver,
		ProfileCacheNotifier: in.ProfileCacheNotifier,
	})
}

func buildInteractionsWorkflow(ctx clientAPIWorkflowContext) clientapiUsecases.Interactions {
	in := ctx.in
	return clientapiUsecases.NewInteractions(clientapiUsecases.InteractionsConfig{
		CommonConfig:        ctx.common,
		NotesRepo:           in.NotesRepo,
		AccountsRepo:        in.AccountsRepo,
		MediaRepo:           in.MediaRepo,
		SocialRepo:          in.SocialRepo,
		BoostsRepo:          in.BoostsRepo,
		MentionsRepo:        in.MentionsRepo,
		PollsRepo:           in.PollsRepo,
		RemoteAccountsRepo:  in.RemoteAccountsRepo,
		DomainBlocksRepo:    in.ModerationRepo,
		RemoteResolver:      ctx.remoteResolver,
		IDGenerator:         ctx.idGenerator,
		CreateInteractionUC: ctx.createInteractionUC,
		UndoInteractionUC:   ctx.undoInteractionUC,
		VotePollUC:          ctx.votePollUC,
	})
}

func buildExternalInteractionWorkflow(ctx clientAPIWorkflowContext) clientapiUsecases.ExternalInteraction {
	in := ctx.in
	return clientapiUsecases.NewExternalInteraction(clientapiUsecases.ExternalInteractionConfig{
		CommonConfig:       ctx.common,
		AccountsRepo:       in.AccountsRepo,
		RemoteAccountsRepo: in.RemoteAccountsRepo,
		DomainBlocksRepo:   in.ModerationRepo,
		RemoteResolver:     ctx.remoteResolver,
	})
}

func buildNotificationsWorkflow(ctx clientAPIWorkflowContext) clientapiUsecases.Notifications {
	in := ctx.in
	return clientapiUsecases.NewNotifications(clientapiUsecases.NotificationsConfig{
		CommonConfig:       ctx.common,
		AccountsRepo:       in.AccountsRepo,
		NotesRepo:          in.NotesRepo,
		MediaRepo:          in.MediaRepo,
		SocialRepo:         in.SocialRepo,
		BoostsRepo:         in.BoostsRepo,
		MentionsRepo:       in.MentionsRepo,
		PollsRepo:          in.PollsRepo,
		RemoteAccountsRepo: in.RemoteAccountsRepo,
		DomainBlocksRepo:   in.ModerationRepo,
		RemoteResolver:     ctx.remoteResolver,
	})
}

func buildConversationsWorkflow(ctx clientAPIWorkflowContext) clientapiUsecases.Conversations {
	in := ctx.in
	return clientapiUsecases.NewConversations(clientapiUsecases.ConversationsConfig{
		CommonConfig:       ctx.common,
		AccountsRepo:       in.AccountsRepo,
		NotesRepo:          in.NotesRepo,
		MediaRepo:          in.MediaRepo,
		SocialRepo:         in.SocialRepo,
		BoostsRepo:         in.BoostsRepo,
		MentionsRepo:       in.MentionsRepo,
		PollsRepo:          in.PollsRepo,
		ConversationsRepo:  in.ConversationsRepo,
		RemoteAccountsRepo: in.RemoteAccountsRepo,
		DomainBlocksRepo:   in.ModerationRepo,
		RemoteResolver:     ctx.remoteResolver,
	})
}

func buildMediaWorkflow(ctx clientAPIWorkflowContext) clientapiUsecases.Media {
	return clientapiUsecases.NewMedia(clientapiUsecases.MediaConfig{
		MediaRepo:          ctx.in.MediaRepo,
		MediaStorage:       ctx.in.MediaStorage,
		RemoteMediaFetcher: ctx.remoteMediaFetcher,
		IDGenerator:        ctx.idGenerator,
	})
}

func buildProfileWorkflow(ctx clientAPIWorkflowContext) clientapiUsecases.Profile {
	return clientapiUsecases.NewProfile(clientapiUsecases.ProfileConfig{
		MediaRepo:        ctx.in.MediaRepo,
		MediaStorage:     ctx.in.MediaStorage,
		ContentSanitizer: ctx.in.ContentSanitizer,
		IDGenerator:      ctx.idGenerator,
		UpdateActorUC:    ctx.updateActorUC,
	})
}

func buildModerationWorkflow(ctx clientAPIWorkflowContext) clientapiUsecases.Moderation {
	return clientapiUsecases.NewModeration(clientapiUsecases.ModerationConfig{
		TxProvider:         ctx.in.TxProvider,
		DomainBlocksRepo:   ctx.in.ModerationRepo,
		ModerationJobsRepo: ctx.in.ModerationRepo,
		DomainPurgeRepo:    ctx.in.ModerationRepo,
	})
}
