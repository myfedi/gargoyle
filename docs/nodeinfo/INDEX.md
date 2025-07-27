# Nodeinfo

[https://nodeinfo.diaspora.software/](Nodeinfo) allows discovery by giving out information about our server. It comes in different versions. Which ones we support is communicated via the `/.well-known/nodeinfo` endpoint:

(`curl http://localhost:3001/.well-known/nodeinfo`)

```json
{
    "links": [
        {
            "rel": "http://nodeinfo.diaspora.software/ns/schema/2.0",
            "href": "http://localhost:3001/nodeinfo/2.0"
        },
        {
            "rel": "http://nodeinfo.diaspora.software/ns/schema/2.1",
            "href": "http://localhost:3001/nodeinfo/2.1"
        }
    ]
}
```

The respective versions (2.0 or 2.1) hold further information about our node:

```json
{
    "version": 2.1,
    "links": [
        {
            "rel": "http://nodeinfo.diaspora.software/ns/schema/2.1",
            "href": "http://localhost:3001/nodeinfo/2.1",
            "protocols": ["activitypub"],
            "software": {
                "name": "Gargoyle",
                "version": 0.0.1-beta,
                "homepage": "https://github.com/myfedi/gargoyle",
                "repository": "https://github.com/myfedi/gargoyle"
            },
            "usage": {
                "users": 0,
                "localPosts": 0,
                "localComments": 0,
            }
        }
    ]
}
```

(and we appropriately return it with `Content-Type: application/json; profile="http://nodeinfo.diaspora.software/ns/schema/2.1#"`)

Note that we redirect `/.well-known/nodeinfo/{version}` to `/nodeinfo/{version}` to support more implementations.
