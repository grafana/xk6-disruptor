# xk6-disruptor roadmap

xk6-disruptor is a young project and is still in an early development phase. The initial releases have been focused on providing a MVP (Minimal Viable Product) that show-cases its potential for a relevant use case: Fault injection in the HTTP requests server by a set of Pods.

However, the vision of the project is more ambitions. We envision to have a tool that democratizes the reliability testing by offering application developers, testers, QA engineers, Devops, SREs and any one concerned with reliability, a simple way to test applications under multiple disruptive conditions.

Our roadmap includes therefore objectives to extend the xk6-disruptor to new use cases, but also to improve existing functionalities making them easier to use and more robust.

## Short-term

These are goals we expect to achieve in the next 3-6 months (Q4/2022-Q1/2023).

1. Expand the catalog of faults for Pod and Service disruptors

   - Connection dropping

        Applications generally maintain a pool of open connections for accessing other services they depend on (for example a database). Different situations, such as resource exhaustion, can force these connections to be disconnected by the service end. The connection dropping fault will simulate this situation forcing a fraction of connections served by a Pod to disconnect, allowing developers to test the application's ability of detect and recover from these disconnections.

   - Random pod kill

        Even when Kubernetes takes care of automatic restarting failed Pods, there are situations on which the recovery from a Pod crash may introduce failures in the application. Some examples may be the recovery of a member of a stateful set that requires re-synchronization with other members of the set. The random pod kill will forcefully terminate a fraction of the pods selected by a selector.

2. Expand the catalog of disruptors

    - NodeDisruptor

      A NodeDisruptor targets a set of nodes, selected or excluded by labels, annotations and state, and support the injection of node-level faults, such a resource exhaustion and network disruption.

3. Improve API

    The main tenet of xk6-disruptor is to offer the best developer experience. In this regard, we will continue improving the API, providing more simplicity and convenience (making the general use cases easier) and extensibility (making complex use cases possible).

    - Add more criteria for Pod selection

        Presently the pod selector only supports labels. It would be convenient to also select or exclude Pods based on annotations or their state (for example, exclude pods that are in Terminated state)

    - Minimize the boiler-plate configuration for injecting faults

        Even when the xk6-disruptor has a lean API for selecting targets and defining the faults, the execution of the fault injection code requires some additional configuration. For example, defining scenarios for executing the fault injection. If a test does not have scenarios, this is considerable overhead. We want to explore alternatives such as helper classes for generating the required configuration using some sound defaults which could be easily overridden.

        Follow-up issues:
        - https://github.com/grafana/xk6-disruptor/issues/54

    - Improve validations in the API

        Presently the API in Go code does not validate the parameter passed from the test script. This introduces several issues, including the difficulty for users to detect when they misspell arguments. We plan to address this issue by creating an API layer between the test code (JavaScript) and the extension implementation (Go) that will validate the parameters passed to any function.

        Follow-up issues:
        - https://github.com/grafana/xk6-disruptor/issues/45

## Mid-term

These are goals we expect to achieve in 6-12 months (Q2/2023-Q3/2023).

1. Add fault injection capabilities for other protocols

   Presently the disruptor only supports fault injection for HTTP protocol. However, many microservice applications use gRPC. Additionally, the ability to inject faults in database connections (e.g., Redis, MySQL) is relevant for many use cases.
   Therefore, we plan to research available multi-protocol proxies and study how they could be incorporated in the architecture of the disruptor agent.

   Follow-up issues:
   - [Implement fault injection for grpc services](https://github.com/grafana/xk6-disruptor/issues/121)

2. Implement disruption for outgoing requests

   It is a common use case to test the effect of known patterns of behavior in external dependencies (services that are not under the control of the organization). Using the xk6-disruptor, this could be accomplished by implementing a Dependency Disruptor, which instead of disrupting a service (or a group of pods), disrupts the requests these pods make to other services. This could be implemented using a similar approach used by the disruptor: inject a transparent proxy but in this case for outgoing requests.

   Follow up issues:
   - https://github.com/grafana/xk6-disruptor/issues/53


3. Implement interface between xk6-disruptor extension and the disruptor agent using grpc

   Presently, this interface is implemented by executing commands in the agent's container. This is a handy option because it does not require the agent to be accessible outside the Kubernetes cluster (exec command uses Kubernetes API server as a gateway) and will probably stay as a default communication mechanism. However, it has some important limitations in terms of error handling and execution of asynchronous tasks. gRPC would offer a more robust foundation for extension/agent communication. It could still be used by tests running inside the Kubernetes cluster, for example, using the [k6-operator](https://github.com/grafana/k6-operator).

   Follow-up issues:
   - https://github.com/grafana/xk6-disruptor/issues/52


## Non-goals

As any other project, it is important for xk6-disruptor to have a clear boundary of what problems we want to solve and what problems we consider are outside or vision:

1. Fuzzing

   Testing corrupted or invalid responses is an important part of reliability testing.  However, generating realistic responses that are still invalid may require some application specific logic that must be implemented somehow in the disruptor agent. We consider the complexity introduced by this requirement outweigh the benefits.
