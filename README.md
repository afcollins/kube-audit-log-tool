# kbx - kube-burner explorer
By: Andrew Collins

kbx is a Terminal UI application written in Golang for exploring
* [kube-burner](https://kube-burner.github.io/kube-burner) locally-indexed metrics
* kubernetes API server audit events

Build it:
```
go build
```

Run it:
```
./kbx                        # Opens file browser, expects you have .log , .log.gz, .json , .json.gz in the current directory
./kbx audit-events.log.gz    # Opens a single or multiple kapi audit event log files
./kbx containerCPU.json      # Opens a single or multiple kube-burner local-indexing files
```

This tool CAN:
* Open multiple types of metrics files at once. Filter them via 'metricName' facet. Press 'f' to show/hide secondary row for filtering.
* Open metricsProfiles metrics and podLatencyMeasurements in the same session
* Open plaintext and gzipped files

This tool CANNOT:
* Open audit and metrics files in the same session, as they are fundamentally different formats and render differently. One at a time for now, please.