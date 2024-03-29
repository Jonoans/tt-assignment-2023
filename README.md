# TikTok Instant Messaging Assignment

![Tests](https://github.com/Jonoans/tt-assignment-2023/actions/workflows/test.yml/badge.svg)

In this assignment, the task was to create an instance messaging service that complied with the IDL defined in the `idl_http.proto`. To that end, only the backend was required in this assignment. Moreover, an extensive boilerplate code was already provided.

# Overall System

The messaging service has 2 primary components.
1. A HTTP service whose purpose would be to perform service discovery and forward requests it receives to the backend RPC service.
2. An RPC service which will process a request to either send or pull messages, interacting with a given database to perform it's task.

These 2 services will use protocol buffers to conduct the RPC requests.

The database I have chosen to use is PostgreSQL, primarily due to familiarity.

# Setup Instruction
This setup instruction assumes that [Docker](https://docs.docker.com/get-docker/) and [Docker Compose](https://docs.docker.com/compose/install/) is already installed.

The startup of the services may take a while as a heathcheck is configured to ensure the database is properly started before the RPC service can start.

```bash
git clone https://github.com/Jonoans/tt-assignment-2023
cd tt-assignment-2023
docker-compose up -d --build
```

# Changes Made

## HTTP Service

Since I viewed the HTTP service as more of a forwarder which worked as an intermediary which performed the service discovery steps, the changes to this service was relatively minor.

1. `PullResponseRest` struct was added and is passed as the struct to be serialised to the final JSON response.<br>
    This struct was added to ensure that the `messages` and `hasMore` field were explicitly set in the JSON response body. Before, they were not set as the struct generated by the protobuf compiler had automatically included the `omitempty` struct tag which caused the `messages` and `hasMore` field to be unset in the returned JSON response.

2. `SendTime` field is set in here to reflect more closely the time at which the message was sent by the user.
    `time.Now().UTC().UnixMicro()` is used to retrieve the current UTC epoch time in microseconds.

3. The `client.WithHostPorts` option passed during the creation of the RPC client was removed.<br>
    This was to allow to automatic retrieval of service IP information from the etcd service discovery service facilitated by the `github.com/kitex-contrib/registry-etcd` library.

## RPC Service

Most of the functionality involving the service was implemented in RPC service, thus changes to this portion of the code is much greater, thus rather than delving in specifics of the changes as in the HTTP service section, I will instead summarise them.

For starters, PostgreSQL was the database I picked to back this message service. Thus, the popular GORM package to interface with the database.

There were 2 data structs that were created to support this service, first the `ChatMessage` struct used to store the chat messages in the database and next the `ChatCusorCache` struct used to "cache" some information to help facilitate quicker lookups for messages.
This will be expounded in the later section.

Indexes are used to support quicker lookups. These indexes are defined using the `gorm` struct tags.

### Send Function

1. Incoming messages are first validated based on the expected message format.
2. Once the incoming message is presumed valid, the actual message is stored (mostly as is, except leading and trailing spaces in `text` is removed) in the database.
3. Then, stale cache information (due to the new incoming message) is deleted.
4. The above database operations are done in a single transaction to ensure atomicity of the operations.

### Pull Function

This function supports the lookup and retrieval of messages from the database. Since pagination is required in this service, limit and offset queries are used to implement the pagination. However, there are limitations with such an implementation. Namely, if cursor is arbitarily large as the client retrieves a great number of messages in successive paginated queries, the database queries will become expensive and slow. Thus, data in the `ChatCursorCache` struct is used to save some useful information that may help support paginated queries.

The assumption is that the paginated queries will come in a rather predictable fashion, where one will first lookup the first 10 messages, then the next 10 and so on.

Thus, in the implementation of the lookup, the service always attempts to retrieve 1 more message than is required (i.e. if first 10 messages are required, it will attempt to retrieve the first 11). This helps us to both determine if there are any more available messages after the first 10 (to populate `hasMore`) and construct a `ChatCursorCache` struct to be saved to the database with information such as the SendTime of the 11th message and current request context (whether it is a reverse chronological lookup) to support our lookup.

The information "cached" in the database helps us to use the `SendTime (>=|<=) ?` condition to filter messages when we retrieve the next 10 messages (11th message will be the first message, whose `SendTime` we have cached) before performing a limit on the number of returned results. This helps us avoid the use of offset.

1. Incoming requests are validated to ensure `limit` and `cursor` are >= 0. Limit will be presumed to be the default of 10 if it is set to 0.<br>
    The `chat` field is also validated to ensure it is in the expected format of `member1:member2`
2. For lookups, the service will default to limit and offset queries if cursor == 0 or if cache information required is not available. Otherwise it will attempt to query the requested information with conditionals and limit.

### Other Changes

Some additional code was also added in the main function to support service discovery features. The added code attempts to use the hostname of the service's deployment environment to lookup its own IP address. This IP address will then be registered as the service instance's IP address on the registry.

In the case of our deployment environment, such a method will return a valid IP address which other containers in the setup can use to access our RPC server instance. However, such a method may not work in other environments.
