
# Design Doc: Redesign Fault Injection API

|                       |               | 
|-----------------------|---------------|
|**Author(s)**:         | @pablochacin |
|**Created**:           | 2023-03-16 |
|**Status**:            | Draft |
|**Last status change**:| 2023-03-16 |
|**Approver(s)**:       | N/A |
|**Related**| N/A |
|**Replaces**| N/A |
|**Superseded by** | N/A |


## Background

The xk6-disruptor API is built around the concept disruptors that inject faults. Presently, each disruptor implements one method for each type of fault it injects. For example, `ServiceDisruptor` and `PodDisruptor` both implement the `InjectHTTPFault` method.

In the reminder of this document we make more emphasis on the existing duplicity between the `ServiceDisruptor` and the `PodDisruptor`, but the same duplicity may occur between other unrelated disruptors. For example, network level disruptions may be injected to both pods and nodes.

This design is convenient because each disruptor can modify or enhance the behavior of their fault injection methods. For example, `InjectHTTFaults` takes the port definition from the service and uses it to locate the target port in the pods for the http faults. For the `PodDisruptor` the `port` indicates the Pod port, while for the `ServiceDisruptor` it indicates the service port.

The [API]https://github.com/grafana/xk6-disruptor/blob/main/pkg/api/api.go serves as a bridge between Javascript code and the disruptors implemented by the extension (golang). It must validate and convert the objects received from/returned to Javascript and delegates the execution to the disruptor. 


## Problem statement

Replicating the method for fault injection in multiple disruptors creates significant duplication in the implementation and the extension and in the documentation.

## Goals

1. Reduce redundancies in the implementation of the API
2. Reduce redundancies in documentations to ensure consistency

### Non-goals (optional)

N/A

### Out of scope

1. Consider a redesign of the API around other concepts than disruptors and faults.

## Alternatives

### Create a generic fault injection API

Redesign the API around a generic `InjectFault` function implemented by all disruptors. This function receives the description of the fault as a object. The documentation for each disruptor class must lists which types of faults it support and document any difference in the way they handle these faults. The fault object which is documented separately and this description is shared by all disruptors supporting it.

```javascript
Disruptor.injectFault(type, fault, duration)
```



### Advantages
1. Simplified API design: only one function that checks fault parameters and dispatches the call to the disruptor. Given that the [current implementation of validation is generic](https://github.com/grafana/xk6-disruptor/blob/main/pkg/api/validation.go), this implementation would be rather straightforward
2. Conciseness of documentation. No need to document a different function for each fault injection.

### Disadvantages

1. It necessary to be careful to document on each class the supported faults and any difference in how default parameters are handled. For example, when injecting HTTPFaults in a service disruptor the port specified in the fault is used in a different way than when applied to a pod disruptor.


### Create fault injection types

The duplicity can be mitigated by creating generic interfaces that implement the fault injection methods. For example, the `HTTPFaultInjector` interface that defines the `InjectHTTPFaults` method and `PodDisruptor` and `ServiceDisruptor` could both implement it.

Similarly `PodDisruptor` and `NodeDisruptor` could both implement he `NetworkFaultInjector` interface that defines the `InjectNetworkFault` method.

The API can then offer implementations for those interfaces that validate and convert the arguments received from JavaScript and delegates the execution to the corresponding disruptor.


See annex [Fault Injectors API](#annex-fault-injectors-api) for a detailed example.


#### Advantages

1. Reduce code duplicity between disruptors that implement the same fault injection

#### Disadvantages

1. This structure of types is rather complex and will grow as new disruptors and faults are added.

### Do nothing

#### Advantages

1. Facilitates documenting any difference in the way disruptors injects faults. For example, different default values.

#### Disadvantages

1. All the issues detailed in the problem description regarding duplicity in the documentation

## Consensus

TBD

## References

## Annex: Fault Injectors API

This annex shows an example 


### Disruptor 

Disruptor is a generic disruptor interface that all disruptors must implement.

```go
type Disruptor interface {
	Targets() []string
}
```

### HttpFault 

```go
type HttpFault struct{}

type HttpFaultInjector interface {
	InjectHttpFaults(HttpFault) error
}
```

### PodDisruptor

```go
type PodDisruptor struct{}

func (p PodDisruptor) Targets() []string {
	return []string{}
}

func (p PodDisruptor) InjectHttpFaults(HttpFault) error {
	return nil
}
```

### ServiceDisruptor

```go
type ServiceDisruptor struct{}

func (s ServiceDisruptor) Targets() []string {
	return []string{}
}

func (s ServiceDisruptor) InjectHttpFaults(HttpFault) error {
	return nil
}
```

### JS API

The JS API implements the API between JS and the disruptors.

In defines the type `JsDisruptor` that implement the generic `Disruptor` interface:

```go
type JsDisruptor struct {
	d Disruptor
}

func (d JsDisruptor) Targets() []string {
	return d.d.Targets()
}
```

It also defines a type that implements each type of fault injection. For example, `JsHttpFaultInjector` implements `HttpFaultInjector`. 


```go
type JsHttpFaultInjector struct {
	d HttpFaultInjector
}

func (i JsHttpFaultInjector) InjectHttpFaults(fault HttpFault) error {
	return i.InjectHttpFaults(fault)
}
```

Finally, it implements a wrapper for each type of disruptor: `JsPodDisruptor` for `PodDisruptor` and `JsServiceDisruptor` for `ServiceDisruptor`. 
Each wrapper embeds the types that implement the methods required by the wrapper:

```go
type JsPodDisruptor struct {
	JsDisruptor
	JsHttpFaultInjector
}

type JsServiceDisruptor struct {
	JsDisruptor
	JsHttpFaultInjector
}
```

We create a `JsPodDisruptor` using an instance of `PodDisruptor` as follows:

```go
d := PodDisruptor{}
j := JsPodDistuptor{
        JsDisruptor: d,
        JsHttpFaultInjector: d,
}
```

The code below invokes the InjectHttpFaults in the `JsHttpFaultInjector` which delegates to the `PodDisruptor`.

```go
f := HttpFault{}`

j.InjectHttpFaults(fault)
```