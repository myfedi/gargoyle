package activitypub

import "testing"

func TestExtractNoteObjectIncludesHashtagsAndEmojis(t *testing.T) {
	raw := []byte(`{
		"type":"Create",
		"actor":"https://remote.example/users/bob",
		"object":{
			"id":"https://remote.example/notes/1",
			"type":"Note",
			"content":"hello #Gargoyle :party_blob:",
			"attributedTo":"https://remote.example/users/bob",
			"tag":[
				{"type":"Hashtag","name":"#Gargoyle"},
				{"type":"Hashtag","name":"gargoyle"},
				{"type":"Emoji","name":":party_blob:","icon":{"url":"https://remote.example/emoji/party_blob.png"}}
			]
		}
	}`)
	note, ok := ExtractNoteObject(raw)
	if !ok {
		t.Fatal("expected note object")
	}
	if len(note.Hashtags) != 1 || note.Hashtags[0] != "Gargoyle" {
		t.Fatalf("unexpected hashtags: %#v", note.Hashtags)
	}
	if len(note.Emojis) != 1 || note.Emojis[0].Shortcode != "party_blob" || note.Emojis[0].URL != "https://remote.example/emoji/party_blob.png" {
		t.Fatalf("unexpected emojis: %#v", note.Emojis)
	}
}

func TestExtractNoteInfersVisibilityFromAudience(t *testing.T) {
	cases := []struct {
		name string
		to   string
		cc   string
		want string
	}{
		{name: "public", to: `"https://www.w3.org/ns/activitystreams#Public"`, cc: `"https://remote.example/users/bob/followers"`, want: "public"},
		{name: "unlisted", to: `"https://remote.example/users/bob/followers"`, cc: `"https://www.w3.org/ns/activitystreams#Public"`, want: "unlisted"},
		{name: "followers", to: `"https://remote.example/users/bob/followers"`, cc: `[]`, want: "private"},
		{name: "mastodon_direct", to: `"http://gargoyle.test/users/alice"`, cc: `[]`, want: "direct"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw := []byte(`{
				"type":"Create",
				"actor":"https://remote.example/users/bob",
				"object":{
					"id":"https://remote.example/notes/` + tc.name + `",
					"type":"Note",
					"content":"hello",
					"attributedTo":"https://remote.example/users/bob",
					"to":` + tc.to + `,
					"cc":` + tc.cc + `
				}
			}`)
			note, ok := ExtractNote(raw)
			if !ok {
				t.Fatal("expected note")
			}
			if note.Visibility != tc.want {
				t.Fatalf("visibility = %q, want %q", note.Visibility, tc.want)
			}
		})
	}
}

func TestExtractActorObjectIncludesProfileFields(t *testing.T) {
	raw := []byte(`{
		"type":"Update",
		"actor":"https://remote.example/users/bob",
		"object":{
			"id":"https://remote.example/users/bob",
			"type":"Person",
			"preferredUsername":"bob",
			"attachment":[
				{"type":"PropertyValue","name":"Website","value":"<a href=\"https://example.org\">example.org</a>"},
				{"type":"Document","name":"ignored","value":"ignored"}
			]
		}
	}`)
	actor, ok := ExtractActorObject(raw)
	if !ok {
		t.Fatal("expected actor object")
	}
	if len(actor.Fields) != 1 || actor.Fields[0].Name != "Website" || actor.Fields[0].Value == "" {
		t.Fatalf("unexpected fields: %#v", actor.Fields)
	}
}

func TestExtractNoteExplicitVisibilityWinsOverAudienceInference(t *testing.T) {
	raw := []byte(`{
		"type":"Create",
		"actor":"https://remote.example/users/bob",
		"object":{
			"id":"https://remote.example/notes/explicit",
			"type":"Note",
			"content":"hello",
			"visibility":"direct",
			"attributedTo":"https://remote.example/users/bob",
			"to":"https://www.w3.org/ns/activitystreams#Public"
		}
	}`)
	note, ok := ExtractNote(raw)
	if !ok {
		t.Fatal("expected note")
	}
	if note.Visibility != "direct" {
		t.Fatalf("visibility = %q, want direct", note.Visibility)
	}
}
