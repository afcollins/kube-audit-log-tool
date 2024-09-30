# A Kubernetes API Server Audit Log Analysis Tool
By: Andrew Collins

At this time, only some scripts that use `jq`.

Attempting to avoid adding heavyweight dependencies that require a stack of software to process a stream of events.

This is intended to analyze a static audit log file, rather than a real-time streaming solution.

# Future Goals:
* An interactive terminal UI to filter, group, sort
* An improved implementation to process larger volume of logs without catting each one every time. (takes ~1.2s for a 200M log file for each command on an 8-core M3)
