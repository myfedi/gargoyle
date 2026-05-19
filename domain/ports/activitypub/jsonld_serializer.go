package activitypub

type JsonLDSerializer interface {
	Marshall(in any) (string, error)
	Unmarshall(in string) (any, error)
}
