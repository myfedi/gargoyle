package clientapi

import "github.com/gofiber/fiber/v2"

type instanceV1Response struct {
	URI         string `json:"uri"`
	Title       string `json:"title"`
	ShortDesc   string `json:"short_description"`
	Description string `json:"description"`
	Email       string `json:"email"`
	Version     string `json:"version"`
	URLs        struct {
		StreamingAPI string `json:"streaming_api"`
	} `json:"urls"`
	Stats struct {
		UserCount   int `json:"user_count"`
		StatusCount int `json:"status_count"`
		DomainCount int `json:"domain_count"`
	} `json:"stats"`
}

func (h APIHandler) instanceV1(c *fiber.Ctx) error {
	info := h.instanceWorkflow.InstanceInfo()
	resp := instanceV1Response{URI: info.Domain, Title: info.Title, ShortDesc: info.Description, Description: info.Description, Version: info.ServerVersion}
	resp.URLs.StreamingAPI = info.Host
	return c.JSON(resp)
}

type instanceV2Response struct {
	Domain      string `json:"domain"`
	Title       string `json:"title"`
	Version     string `json:"version"`
	SourceURL   string `json:"source_url"`
	Description string `json:"description"`
}

func (h APIHandler) instanceV2(c *fiber.Ctx) error {
	info := h.instanceWorkflow.InstanceInfo()
	return c.JSON(instanceV2Response{Domain: info.Domain, Title: info.Title, Version: info.ServerVersion, Description: info.Description})
}
