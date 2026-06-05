package clientapi

import (
	"encoding/base64"
	"net/url"
	"strings"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
)

func requireAccount(account *models.Account) *domainerrors.DomainError {
	if account == nil {
		return domainerrors.New(domainerrors.ErrUnauthorized, "missing account")
	}
	return nil
}

func AccountIDForRemoteActor(actor string) string {
	return remoteAccountIDPrefix + base64.RawURLEncoding.EncodeToString([]byte(actor))
}

func RemoteActorFromAccountID(id string) (string, error) {
	unescaped, err := url.PathUnescape(id)
	if err != nil {
		return "", err
	}
	id = unescaped
	if !strings.HasPrefix(id, remoteAccountIDPrefix) {
		return id, nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(id, remoteAccountIDPrefix))
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}
