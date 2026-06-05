package clientapi

type workflow struct{}

type Instance struct {
	workflow
	deps InstanceConfig
}

type Accounts struct {
	workflow
	deps AccountsConfig
}

type Statuses struct {
	workflow
	deps StatusesConfig
}

type Timelines struct {
	workflow
	deps TimelinesConfig
}

type Interactions struct {
	workflow
	deps InteractionsConfig
}

type Notifications struct {
	workflow
	deps NotificationsConfig
}

type Conversations struct {
	workflow
	deps ConversationsConfig
}

type Media struct {
	workflow
	deps MediaConfig
}

type Profile struct {
	workflow
	deps ProfileConfig
}

type Moderation struct {
	workflow
	deps ModerationConfig
}
