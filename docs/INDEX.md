# Gargoyle
## Architecture
The ports and adapters (also known as hexagonal) architecture is used with inspiration from domain-driven design (DDD). The architecture is designed to be modular, with clear separation of concerns between the domain logic and the infrastructure.

The typical pattern is to define a domain model (representing data independent of implementation details) as well as ports (interfaces) that define interactions with the domain models. Adapters are implementations of these ports.

The domain use cases define the business logic and are implemented in the `domain/usecases` package. They isolate logic of one "feature" or "use case" of the application. The use cases must not depent on any implementation details and are only scoped to ports or domain models. They mustn't use any outside dependencies or adapters.
An opinionated take surely is that the only ports we model are driven adapters. In a web application we're not winning anything by also abstracting the driver adapters like the web server. Thus, the web entry point is in `cmd/web/server.go` and any supplementing code like the actual web handlers are in `infrastructure/web/`.

- models: data structures
- ports: interfaces that define how the domain logic can be interacted with
- usecases: business logic that uses the ports to interact with the domain models
- adapters: implementations of the ports that interact with the outside world (e.g., repositories, web handlers, etc.)
- infrastructure: provides implementations around configuration, web handlers, logging etc.

What is shared by almost anything in this code base is the way dependencies are passed around. Let's say we have a web handler that needs to access a repository to read something from a database. Normally, we'd just reference or instantiate that right there. With ports and adapters, we just declare that we need something that provides this information (a port) and expect that it's given to us when we create the class.

Let's look at how the web handler for the nodeinfo endpoints is constructed for example.

```go
// Adapters for the nodeinfo handlers.
type NodeInfoHandlerConfig struct {
	UsersRepo     repos.UsersRepository
	PostsRepo     repos.PostsRepository
	CommentsRepo  repos.CommentsRepository
	Domain        string
	ServerVersion string
}

type NodeInfoWebHandler struct {
	cfg NodeInfoHandlerConfig
}

// NewNodeInfoWebHandler creates a new NodeInfoWebHandler with the given dependencies.
func NewNodeInfoWebHandler(cfg NodeInfoHandlerConfig) *NodeInfoWebHandler {
	return &NodeInfoWebHandler{
		cfg: cfg,
	}
}
```

We know that for the `usage` key of the nodeinfo endpoint, we need to fetch the number of users, comments and posts on this server. Thus we need something that can retrieve it from the database. We don't actually care about how it's implemented or how it's retrieving this information or even what database is used. We just want the numbers. (Or in other instances we expect domain models.)

This is a pattern we use throughout the code base. Static data and adapters are given when we create an object like a handler. That way we can easily test things or totally change the adapters (change one database for another for example).