# Hostmeta

The hostmeta discovery endpoint simply returns a bit of XML telling other instances how they can find resource on our server:

(`curl http://localhost:3001/.well-known/host-meta`)

```xml
<?xml version="1.0" encoding="UTF-8"?>
<XRD xmlns="http://docs.oasis-open.org/ns/xri/xrd-1.0"><Link rel="lrdd" type="application/xrd+xml" template="http://localhost:3001/.well-known/webfinger?resource={uri}"></Link></XRD>
```