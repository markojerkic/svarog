# Svarog Go library

`github.com/markojerkic/svarog/svaroglog` forwards logs from a Go application to
Svarog without the [CLI client](../cmd/client/README.md). It installs itself as
the handler for `log/slog` and the standard `log` package. It requires Go
1.27.3 or newer.

```bash
go get github.com/markojerkic/svarog/svaroglog
```

## Usage

Set `SVAROG_CONN_STRING` to the client connection URL from the Svarog web UI
and call `Setup` once at startup. Existing `log/slog` and `log` calls are then
forwarded:

```go
package main

import (
    "context"
    "log"
    "log/slog"

    "github.com/markojerkic/svarog/svaroglog"
)

func main() {
    closeLogging, err := svaroglog.Setup(context.Background())
    if err != nil {
        log.Fatal(err)
    }

    slog.Info("Order created", "order_id", 42)
    log.Print("Health check passed")

    if err := closeLogging(); err != nil {
        log.Printf("Could not flush logs: %v", err)
    }
}
```

`Setup` returns an error when no connection URL is configured. Only one `Setup`
may be active at a time, and it does not install signal handlers.

Call the returned function during shutdown. It waits for JetStream
acknowledgements and returns any drain error. An acknowledgement confirms
acceptance by JetStream, not storage in PostgreSQL. If the application has not
replaced the global loggers in the meantime, the function also restores the
previous ones. Canceling the context passed to `Setup` also starts shutdown.

The [native log demo](../cmd/log-interceptor-demo/main.go) shows a complete
application with signal handling.

## Options

| Option                      | Default                                 | Effect                                                                                                                   |
| --------------------------- | --------------------------------------- | ------------------------------------------------------------------------------------------------------------------------ |
| `WithConnectionString(url)` | `SVAROG_CONN_STRING`                    | Connection URL.                                                                                                          |
| `WithInstanceID(name)`      | `SVAROG_INSTANCE_ID`, then the hostname | Instance name attached to the logs.                                                                                      |
| `WithLevel(level)`          | `slog.LevelInfo`                        | Minimum level forwarded to Svarog. The console level is set separately through its handler.                              |
| `WithConsole(handler)`      | INFO text handler on stderr             | Local console output; `nil` disables it. Do not pass the initial `slog.Default().Handler()`, which can cause a log loop. |
| `WithDropQueueCapacity(n)`  | `1024`                                  | Best-effort queue size, from 1 to 65536. `0` selects a blocking 16-slot queue.                                           |
| `WithDrainTimeout(d)`       | 5 seconds                               | Deadline for draining queued logs during shutdown.                                                                       |

## Queueing

By default, forwarding is best effort. The queue holds up to 1024 records and
16 MiB of encoded data. When either limit is reached, new records are not
forwarded but still reach the console. The library reports rejection counts to
stderr at most once per second and reports any remaining count when it stops.

With `WithDropQueueCapacity(0)`, logging calls wait for space in a 16-slot
queue instead of dropping records.
