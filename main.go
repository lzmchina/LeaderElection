package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

var (
	name       = flag.String("election", "", "The name of the election")
	id         = flag.String("id", "", "The id of this participant")
	namespace  = flag.String("election-namespace", v1.NamespaceDefault, "The Kubernetes namespace for this election")
	ttl        = flag.Duration("ttl", 10*time.Second, "The TTL for this election")
	inCluster  = flag.Bool("use-cluster-credentials", false, "Should this server run in cluster?")
	addr       = flag.Int("port", DefaultPort, "If non-empty, stand up a simple webserver that reports the leader state")
	localDebug = flag.Bool("debug", false, "whether debug in local machine")
	leader     = &LeaderData{}
)

func validateFlags() {
	if len(*id) == 0 {
		log.Fatal("--id cannot be empty")
	}
	if len(*name) == 0 {
		log.Fatal("--election cannot be empty")
	}
}
func main() {
	flag.Parse()
	validateFlags()
	apiHandler := NewAPIHandler(*inCluster, leader)
	router := NewRouter(apiHandler)
	LeaderElector, err := NewLeaderElector(apiHandler.client, *name, *id, *namespace)
	if err != nil {
		log.Fatal("init LeaderElector failed: ", err)

	}
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ch
		klog.Info("Received termination, signaling shutdown")
		cancel()
		syscall.Exit(0)
	}()
	if *localDebug {
		LeaderElector.Run(ctx)
	} else {
		go func() {
			LeaderElector.Run(ctx)
		}()
		log.Fatal(http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", *addr), router))
	}

}
