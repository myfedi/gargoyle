package clientapi

import (
	"reflect"
	"testing"
)

func mustPanic(t *testing.T, name string, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatalf("%s did not panic", name)
		}
	}()
	fn()
}

func TestConstructorsValidateRequiredDependencies(t *testing.T) {
	common := CommonConfig{Host: "https://example.com", Domain: "example.com"}

	cases := []struct {
		name string
		fn   func()
	}{
		{name: "instance common", fn: func() { NewInstance(InstanceConfig{}) }},
		{name: "accounts deps", fn: func() { NewAccounts(AccountsConfig{CommonConfig: common}) }},
		{name: "statuses deps", fn: func() { NewStatuses(StatusesConfig{CommonConfig: common}) }},
		{name: "timelines deps", fn: func() { NewTimelines(TimelinesConfig{CommonConfig: common}) }},
		{name: "interactions deps", fn: func() { NewInteractions(InteractionsConfig{CommonConfig: common}) }},
		{name: "notifications deps", fn: func() { NewNotifications(NotificationsConfig{CommonConfig: common}) }},
		{name: "conversations deps", fn: func() { NewConversations(ConversationsConfig{CommonConfig: common}) }},
		{name: "media deps", fn: func() { NewMedia(MediaConfig{}) }},
		{name: "profile deps", fn: func() { NewProfile(ProfileConfig{}) }},
		{name: "moderation deps", fn: func() { NewModeration(ModerationConfig{}) }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { mustPanic(t, tc.name, tc.fn) })
	}
}

func TestInternalHelperTypesDoNotExposeUseCaseMethods(t *testing.T) {
	cases := []struct {
		name  string
		value any
	}{
		{name: "workflow", value: workflow{}},
		{name: "timelineBuilder", value: timelineBuilder{}},
		{name: "statusLoader", value: statusLoader{}},
		{name: "accountResolver", value: accountResolver{}},
		{name: "mediaCleaner", value: mediaCleaner{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if methods := reflect.TypeOf(tc.value).NumMethod(); methods != 0 {
				t.Fatalf("%s exposes %d exported methods; exported client API methods belong on concrete workflow groups", tc.name, methods)
			}
		})
	}
}
