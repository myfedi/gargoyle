package hostmeta

import "fmt"

type HostMetaHandler struct {
	domain string
}

func NewHostMetaHandler(domain string) *HostMetaHandler {
	return &HostMetaHandler{domain: domain}
}

// HandleHostMetaXML processes the host-meta request for a given domain.
func (h *HostMetaHandler) HandleHostMetaXML() (string, error) {
	hostMeta := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<XRD xmlns="http://docs.oasis-open.org/ns/xri/xrd-1.0"><Link rel="lrdd" type="application/xrd+xml" template="%s/.well-known/webfinger?resource={uri}"></Link></XRD>
`, h.domain)
	return hostMeta, nil
}
