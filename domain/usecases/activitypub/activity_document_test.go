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
