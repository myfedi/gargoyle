# Webfinger

Webfinger is a protocol used to discover information about people or entities. It allows other users or servers to
discover what and who we're hosting.

[Webfinger Spec](https://webfinger.net/)

## Implementation
In `infrastructure/web/handlers/webfinger.go` we implement one endpoint: `/.well-known/webfinger`. It takes
a resource query parameter and returns information about where to get more information about the actor (user/bot).

If you request the following webfinger resource:

```
curl -L -H 'Accept: application/jrd+json' 'https://example.org/.well-known/webfinger?resource=acct:alice@example.org'
```

A possible output would look like this:

```
{
  "subject": "acct:alice@example.org",
  "links": [
    {
      "rel": "self",
      "type": "application/activity+json",
      "href": "https://example.org/users/alice"
    }
  ]
}
```

The resource `https://example.org/users/alice` holds further information about the user and is part of the activitypub standard.