package clientapi

func NewInstance(cfg InstanceConfig) Instance {
	validateCommon(cfg.CommonConfig, "instance")
	return Instance{deps: cfg}
}

func NewAccounts(cfg AccountsConfig) Accounts {
	validateAccountsConfig(cfg)
	return Accounts{deps: cfg}
}

func NewStatuses(cfg StatusesConfig) Statuses {
	validateStatusesConfig(cfg)
	return Statuses{deps: cfg}
}

func NewTimelines(cfg TimelinesConfig) Timelines {
	validateTimelinesConfig(cfg)
	return Timelines{deps: cfg}
}

func NewInteractions(cfg InteractionsConfig) Interactions {
	validateInteractionsConfig(cfg)
	return Interactions{deps: cfg}
}

func NewNotifications(cfg NotificationsConfig) Notifications {
	validateNotificationsConfig(cfg)
	return Notifications{deps: cfg}
}

func NewConversations(cfg ConversationsConfig) Conversations {
	validateConversationsConfig(cfg)
	return Conversations{deps: cfg}
}

func NewMedia(cfg MediaConfig) Media {
	validateMediaConfig(cfg)
	return Media{deps: cfg}
}

func NewProfile(cfg ProfileConfig) Profile {
	validateProfileConfig(cfg)
	return Profile{deps: cfg}
}

func NewModeration(cfg ModerationConfig) Moderation {
	validateModerationConfig(cfg)
	return Moderation{deps: cfg}
}

func validateCommon(cfg CommonConfig, name string) {
	if cfg.Host == "" || cfg.Domain == "" {
		panic("client API " + name + " workflow requires Host and Domain")
	}
}

func validateAccountsConfig(cfg AccountsConfig) {
	validateCommon(cfg.CommonConfig, "accounts")
	if cfg.AccountsRepo == nil || cfg.NotesRepo == nil || cfg.FollowsRepo == nil || cfg.MediaRepo == nil || cfg.MediaStorage == nil || cfg.RemoteMediaFetcher == nil || cfg.SocialRepo == nil || cfg.BoostsRepo == nil || cfg.MentionsRepo == nil || cfg.PollsRepo == nil || cfg.RemoteAccountsRepo == nil || cfg.DomainBlocksRepo == nil || cfg.IDGenerator == nil || cfg.RemoteResolver == nil {
		panic("client API accounts workflow missing repository or resolver dependency")
	}
}

func validateStatusesConfig(cfg StatusesConfig) {
	validateCommon(cfg.CommonConfig, "statuses")
	if cfg.NotesRepo == nil || cfg.AccountsRepo == nil || cfg.MediaRepo == nil || cfg.MediaStorage == nil || cfg.SocialRepo == nil || cfg.BoostsRepo == nil || cfg.MentionsRepo == nil || cfg.PollsRepo == nil || cfg.DomainBlocksRepo == nil || cfg.RemoteAccountsRepo == nil || cfg.RemoteResolver == nil || cfg.ContentSanitizer == nil || cfg.IDGenerator == nil {
		panic("client API statuses workflow missing repository or service dependency")
	}
}

func validateTimelinesConfig(cfg TimelinesConfig) {
	validateCommon(cfg.CommonConfig, "timelines")
	if cfg.NotesRepo == nil || cfg.AccountsRepo == nil || cfg.FollowsRepo == nil || cfg.MediaRepo == nil || cfg.SocialRepo == nil || cfg.BoostsRepo == nil || cfg.MentionsRepo == nil || cfg.PollsRepo == nil || cfg.RemoteAccountsRepo == nil || cfg.DomainBlocksRepo == nil || cfg.RemoteResolver == nil {
		panic("client API timelines workflow missing repository dependency")
	}
}

func validateInteractionsConfig(cfg InteractionsConfig) {
	validateCommon(cfg.CommonConfig, "interactions")
	if cfg.NotesRepo == nil || cfg.AccountsRepo == nil || cfg.MediaRepo == nil || cfg.SocialRepo == nil || cfg.BoostsRepo == nil || cfg.MentionsRepo == nil || cfg.PollsRepo == nil || cfg.RemoteAccountsRepo == nil || cfg.DomainBlocksRepo == nil || cfg.RemoteResolver == nil || cfg.IDGenerator == nil {
		panic("client API interactions workflow missing repository or service dependency")
	}
}

func validateNotificationsConfig(cfg NotificationsConfig) {
	validateCommon(cfg.CommonConfig, "notifications")
	if cfg.AccountsRepo == nil || cfg.NotesRepo == nil || cfg.MediaRepo == nil || cfg.SocialRepo == nil || cfg.BoostsRepo == nil || cfg.MentionsRepo == nil || cfg.PollsRepo == nil || cfg.RemoteAccountsRepo == nil || cfg.DomainBlocksRepo == nil || cfg.RemoteResolver == nil {
		panic("client API notifications workflow missing repository dependency")
	}
}

func validateConversationsConfig(cfg ConversationsConfig) {
	validateCommon(cfg.CommonConfig, "conversations")
	if cfg.AccountsRepo == nil || cfg.NotesRepo == nil || cfg.MediaRepo == nil || cfg.SocialRepo == nil || cfg.BoostsRepo == nil || cfg.MentionsRepo == nil || cfg.PollsRepo == nil || cfg.ConversationsRepo == nil || cfg.RemoteAccountsRepo == nil || cfg.DomainBlocksRepo == nil || cfg.RemoteResolver == nil {
		panic("client API conversations workflow missing repository dependency")
	}
}

func validateMediaConfig(cfg MediaConfig) {
	if cfg.MediaRepo == nil || cfg.MediaStorage == nil || cfg.IDGenerator == nil {
		panic("client API media workflow missing media dependency")
	}
}

func validateProfileConfig(cfg ProfileConfig) {
	if cfg.MediaRepo == nil || cfg.MediaStorage == nil || cfg.ContentSanitizer == nil || cfg.IDGenerator == nil {
		panic("client API profile workflow missing media or sanitizer dependency")
	}
}

func validateModerationConfig(cfg ModerationConfig) {
	if cfg.TxProvider == nil || cfg.DomainBlocksRepo == nil || cfg.ModerationJobsRepo == nil || cfg.DomainPurgeRepo == nil {
		panic("client API moderation workflow missing repository or transaction dependency")
	}
}
