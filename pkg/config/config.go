/*
Copyright 2019 Baidu, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package config defines data structure needed by cluster controller,
// and const of cluster controller.
package config

import (
	otev1 "github.com/baidu/ote-stack/pkg/apis/ote/v1"
	oteclient "github.com/baidu/ote-stack/pkg/generated/clientset/versioned"
)

const (
	// ROOT_CLUSTER_NAME defines the cluster name of root cluster.
	ROOT_CLUSTER_NAME = "Root"

	// CLUSTER_CONNECT_HEADER_LISTEN_ADDR defines request header to post listen address of the child.
	// For edge when connect to parent, set this header to address listend by the cluster,
	// so let parent know the address of child, thus a cluster can get its neighbor from parent.
	CLUSTER_CONNECT_HEADER_LISTEN_ADDR = "listen-addr"

	// K8S_INFORMAER_SYNC_DURATION defines k8s informer sync seconds.
	K8S_INFORMAER_SYNC_DURATION = 10
)

// ClusterControllerConfig contains config needed by cluster controller.
type ClusterControllerConfig struct {
	TunnelListenAddr  string
	ParentCluster     string
	ClusterName       string
	KubeConfig        string
	HelmTillerAddr    string
	RemoteShimAddr    string
	K8sClient         oteclient.Interface
	EdgeToClusterChan chan otev1.ClusterController
	ClusterToEdgeChan chan otev1.ClusterController
}

// ClusterRegistry defines a data structure to use when a cluster regists.
type ClusterRegistry struct {
	Name   string
	Listen string
}

// IsRoot check if clusterName is a root cluster.
func IsRoot(clusterName string) bool {
	return ROOT_CLUSTER_NAME == clusterName
}
