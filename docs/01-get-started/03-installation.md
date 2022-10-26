
# Installation

## Built from source

Before building a custom `k6` image that contains the `xk6-disruptor` extension ensure you have [Go 1.18](https://golang.org/doc/install) and [Git](https://git-scm.com/) installed.

Once these requirements are satisfied, you will also need to install the [xk6 build tool](https://github.com/grafana/xk6#command-usage):
```bash
$ go install go.k6.io/xk6/cmd/xk6@latest
```

Then you will need to clone the source code from the [k6s-disruptor](https://github.com/grafana/xk6-disruptor) repository:
```bash
$ git clone https://github.com/grafana/xk6-disruptor.git
$ cd xk6-disruptor
```

The custom binary can be then built my executing the following command:
```bash
$ xk6 build --with github.com/grafana/xk6-disruptor=. --with github.com/grafana/xk6-kubernetes --output build/k6
```

Notice that we are including both the `xk6-disruptor` and the [xk6-kubernetes extension](https://github.com/grafana/xk6-kubernetes) when building the custom `k6` binary using the command above. This is because many example scripts use the `xk6-kubernetes` extension for creating the Kubernetes resources they need, such as Pods and Services. If you don't use this extension in your tests you can build the custom `k6` binary with only the `xks-disruptor` extension using the following command instead:
```bash
$ xk6 build --with github.com/grafana/xk6-disruptor=. --output build/k6
```

The test scripts can be executed then running the newly created version of `k6` located in the `build` directory:
```bash
$ ./build/k6 run path/to/test/script
```
