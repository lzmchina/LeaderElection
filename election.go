package main

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	rl "k8s.io/client-go/tools/leaderelection/resourcelock"
)

func createLeaseLock(client *kubernetes.Clientset, electionId string, nodeId string, namespace string) *rl.LeaseLock {
	return &rl.LeaseLock{LeaseMeta: metav1.ObjectMeta{
		Name:      electionId,
		Namespace: namespace,
	},
		Client: client.CoordinationV1(),
		LockConfig: rl.ResourceLockConfig{
			Identity: nodeId,
		}}
}
func NewLeaderElector(client *kubernetes.Clientset, electionId string, nodeId string, namespace string) (*leaderelection.LeaderElector, error) {
	// 指定锁的资源对象，这里使用了Lease资源，还支持configmap，endpoint，或者multilock(即多种配合使用)
	lock := createLeaseLock(client, electionId, nodeId, namespace)
	config := leaderelection.LeaderElectionConfig{
		Lock:            lock,
		ReleaseOnCancel: true,
		LeaseDuration:   15 * time.Second,
		RenewDeadline:   10 * time.Second,
		RetryPeriod:     2 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(c context.Context) {
				fmt.Printf("现在%s成为leader\n", nodeId)
				//leader.Name = nodeId
			},
			OnStoppedLeading: func() {
				fmt.Printf("现在%s不是leader\n", nodeId)
			},
			OnNewLeader: func(identity string) {
				leader.Name = identity
				fmt.Printf("发现%s成为了leader\n", nodeId)
			},
		},
	}
	return leaderelection.NewLeaderElector(config)
}
