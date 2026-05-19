package activitypub

import (
	"encoding/json"
	"log/slog"

	"sourcery.dny.nu/pana"
)

type PanaJsonLDSerializer struct {
	processor *pana.Processor
	context   json.RawMessage
}

func NewPanaJsonLDSerializer() PanaJsonLDSerializer {
	return PanaJsonLDSerializer{
		processor: pana.NewProcessor(slog.New(slog.DiscardHandler)),
		context:   json.RawMessage(`{"@context":"https://www.w3.org/ns/activitystreams"}`),
	}
}

// var _ = (activitypub.JsonLDSerializer) Pana

func (s PanaJsonLDSerializer) Marshall(activity pana.Activity) (string, error) {
	compacted, err := s.processor.Marshal(
		s.context,
		activity,
	)
	if err != nil {
		return "", err
	}

	return string(compacted), nil
}

func (s PanaJsonLDSerializer) Unmarshall(in string) (any, error) {
	return nil, nil // FIXME:
}
