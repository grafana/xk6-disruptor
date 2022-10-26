# Architecture

The xk6-disruptor consists of two main components: a k6 extension and the xk6-disruptor-agent. The xk6-disruptor-agent is a command line tool that can inject disruptions in the target system where it runs. The xk6-disruptor extension provides an API for injecting faults into a target system using the xk6-disruptor as a backend tool. The xk6-disruptor extension will install the agent in the target and send commands in order to inject the desired faults.

 The xk6-disruptor-agent is provided as an Docker image that can be pulled from the [xk6-disruptor repository](https://github.com/grafana/xk6-disruptor/pkgs/container/xk6-disruptor-agent) as or [build locally](./01-contributing.md#building-the-xk6-disruptor-agent-image).

## xk6-disruptor-agent

The agent offers a series of commands that inject different types of disruptions described in the next sections.

### HTTP

The http command injects disruptions in the requests sent to a target http server.
The target is defined by the tcp port and interface where the target is listening.
The disruptions are defined as either delays in the responses and/or a rate of errors
returned from the request.

The following command shows the options in detail:
```sh
$ xk6-disruptor-agent http -h
Disrupts http request by introducing delays and errors. Requires NET_ADMIM capabilities for setting iptable rules.

Usage:
  xk6-disruptor-agent http [flags]

Flags:
  -a, --average-delay uint     average request delay (milliseconds) (default 100)
  -v, --delay-variation uint   variation in request delay (milliseconds
  -d, --duration duration      duration of the disruptions (default 1m0s)
  -e, --error uint             error code
  -x, --exclude stringArray    path(s) to be excluded from disruption
  -h, --help                   help for http
  -i, --interface string       interface to disrupt (default "eth0")
  -p, --port uint              port the proxy will listen to (default 8080)
  -r, --rate float32           error rate
  -t, --target uint            port the proxy will redirect request to (default 80)
```