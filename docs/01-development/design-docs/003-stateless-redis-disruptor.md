
# Design Doc: Title

|                         |                                                          | 
|-------------------------|----------------------------------------------------------|
| **Author(s)**:          | Roberto Santalla (@roobre)                               |
| **Created**:            | 2023-09-06                                               |
| **Status**:             | Draft                                                    |
| **Last status change**: | 2023-09-06                                               |
| **Approver(s)**:        | Pablo Chacín (@pablochacin), Daniel González (@dgzlopes) |
| **Related**             |                                                          |
| **Replaces**            |                                                          |
| **Superseded by**       |                                                          |


## Background

Caching services like Redis are a common way to improve the performance of distributed systems, but sometimes make difficult to know or estimate how a given system will behave when the caching service stops behaving as expected. Non-catastrophic failure modes such as increase of latency, or unexpected miss rate increase can affect a distributed system in qualitative ways and lead to catastrophic failure. 

## Problem statement

A big reliability challenge, and a common source of incidents, is the [metastable behavior](http://charap.co/metastable-failures-in-distributed-systems/) ([archive.org](https://web.archive.org/web/20230502171335/http://charap.co/metastable-failures-in-distributed-systems/)) of distributes systems when using common patterns such as caching. A common example of a metastable failure is system that is responding well to a certain load thanks to warm cache, but when this cache is lost due to an instance restart, or a node failure, the sudden cache miss events overload the backing database preventing the system from recovering.

Most systems architects are aware of this problem qualitatively, but not quantitatively: Which is the maximum Redis latency the system can tolerate? Below which miss rate the load on the database is enough to start dropping requests? These are questions that are hard to answer with the current tooling. 

## Goals

Add baseline redis faulting functionality to the disruptor, so application and platform engineers can perform tests and understand how their systems respond to non-ideal conditions. This document proposes the addition of two types of faults:

- Delay faults, where a delay is added to the time that would normally take for a client to receive a response for a given command
- Miss rate faults, where a certain percentage of keys will be simulated to not exist on the server from the client's perspective.

### Out of scope

Faults related to Redis when it is given other roles than as a caching key-value store.

## Proposal

This document proposes to create a stateless Redis protocol (RESP2 and RESP3) proxy. RESP is a binary, but ascii-based protocol built on top of TCP. The protocol is relatively simple to parse, being clearly delimited by a known separator (`\r\n`). [Bulk strings](https://redis.io/docs/reference/protocol-spec/#bulk-strings) are the only type that may pose some parsing challenges.

RESP is pipelined, which means that the same connection may be used for the client to send multiple requests, without waiting for the server to respond to each one. This means that for a proxy to be able to know to which request a response belongs, it needs to keep in memory the request that originated it. Such a proxy is referred in this document as a stateful proxy, as it needs to keep in memory the state of the command queue, and introduces complexity to the proxy.

Without the requirement of being able to correlate responses with the requests that originated them, a RESP proxy can be made stateless. This reduces the complexity at the cost of, as expected, not being to correlate those responses. However, it should still be possible to meet the goals above with an stateless proxy.

A stateless RESP proxy accepts connections from Redis clients. It will read messages sent by clients, parse them, and decide if any action is necessary, such as modifying the request, or delaying it. It simply passes through responses from the server back to the client, without needing to decode them. A stateless proxy always needs to forward requests, modified or not, to the upstream server. As it is not aware of the flow of responses, it should be compatible with server pushes without needing any additional logic. 

A stateless RESP proxy can be used as a first step to meet the goals above, with some limitations.

### Stateless delay

For each message received by the RESP proxy proxy, it will wait a certain amount of time before forwarding it upstream. To preserve the protocol's semantics, latency would likely need to be added per-message, even if it includes multiple commands.

As the stateless proxy does not match requests to responses, latency will always be added to the upstream server latency.

### Stateless miss rate

For each command that retrieves one or more keys, for each key that matches a user-specified prefix a random number will be generated and compared to a user-specified threshold. If the number is smaller than the threshold, the key is modified to a randomly generated value that will most likely not exist.

For example, the if the following commands arrive on a batch:
```
GET users:1234
GET users:1235
GET users:1236
```

And we are faking a 33% miss rate, the proxy would modify that batch and send the following upstream:
```
GET users:1234
GET __xk6_disrupted_1693995714__
GET users:1236
```

#### Caveats

A stateless proxy cannot inject fully consistent miss rate faults, as it wouldn't be able to affect commands used to list the keys present in Redis such as:
- `KEYS`
- `RANDOMKEY`
- `SCAN`

This needs to be acknowledged as a limitation of the stateless approach.

### Advantages

- Easier to implement and less error-prone than a stateful proxy

### Disadvantages

- Functionality is more limited and might create edge cases

## Alternatives

### Stateful proxy

A stateful proxy is fully capable of modifying responses is a more capable, but more complex alternative. A stateful proxy is able to link requests and responses by pushing request to a queue as they are sent upstream, and removing elements from the queue for each response that comes back. As a result, requests are buffered in memory, which increases resource usage.

#### Advantages

- Proxy has more fine-grained control, and can:
  - Intercept requests directly, generating a response for them without forwarding them to the upstream server
  - Modify responses depending on what was requested, allowing to inject miss rate faults in commands like `KEYS`

#### Disadvantages

- Code is more complex, requiring more development time and increasing the surface for bugs to appear.

### Do nothing

Users won't be able to use xk6-disruptor to test for latency and metastability failures in their distributed systems. 

## Consensus

> To be discussed

## References

- [Redis protocol spec](https://redis.io/docs/reference/protocol-spec)
- [Redis request pipelining](https://redis.io/docs/manual/pipelining/)
