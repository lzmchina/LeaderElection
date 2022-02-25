## kubernetes leader election
use kubernetes leases for leader election
## how to use
### 1. build binary
go build
### 2. run it
leaderElection -h

```bash
Usage of ./leaderElection:
  -debug
        whether debug in local machine
  -election string
        The name of the election
  -election-namespace string
        The Kubernetes namespace for this election (default "default")
  -id string
        The id of this participant
  -port int
        If non-empty, stand up a simple webserver that reports the leader state (default 4040)
  -ttl duration
        The TTL for this election (default 10s)
  -use-cluster-credentials
        Should this server run in cluster?
```
#### 2.1 local run test
leaderElection --election {selected lease name} --election-namespace {lease's namespace} --id {The id of this participant} --debug true
#### 2.2 run in pod container
leaderElection --election {selected lease name} --election-namespace {lease's namespace} --id {The id of this participant}

### curl it
`curl {ip}:4040/get` you will get current leader id
